package config

import (
	"bytes"
	"dario.cat/mergo"
	"errors"
	"os"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

var DefaultBootCommand = "/sbin/boot"

func DefaultBaseImage() string {
	if runtime.GOARCH == "arm64" {
		return "discourse/base:aarch64"
	}
	return "discourse/base:2.0.20231004-0028"
}

type DockerComposeYaml struct {
	Services ComposeAppService
	Volumes  map[string]*interface{}
}
type ComposeAppService struct {
	App ComposeService
}
type ComposeService struct {
	Build       ComposeBuild
	Volumes     []string
	Links       []string
	Environment map[string]string
	Ports       []string
}
type ComposeBuild struct {
	Dockerfile string
	Labels     map[string]string
	Shm_Size   string
	Args       []string
	No_Cache   bool
}

type Config struct {
	Name            string `yaml:-`
	rawYaml         []string
	Base_Image      string            `yaml:,omitempty`
	Update_Pups     bool              `yaml:,omitempty`
	Run_Image       string            `yaml:,omitempty`
	Boot_Command    string            `yaml:,omitempty`
	No_Boot_Command bool              `yaml:,omitempty`
	Docker_Args     string            `yaml:,omitempty`
	Templates       []string          `yaml:templates,omitempty`
	Expose          []string          `yaml:expose,omitempty`
	Params          map[string]string `yaml:Params,omitempty`
	Env             map[string]string `yaml:env,omitempty`
	Labels          map[string]string `yaml:labels,omitempty`
	Volumes         []struct {
		Volume struct {
			Host  string `yaml:host`
			Guest string `yaml:guest`
		} `yaml:volume`
	} `yaml:volumes,omitempty`
	Links []struct {
		Link struct {
			Name  string `yaml:name`
			Alias string `yaml:alias`
		} `yaml:link`
	} `yaml:links,omitempty`
}

func (config *Config) loadTemplate(templateDir string, template string) error {
	content, err := os.ReadFile(strings.TrimRight(templateDir, "/") + "/" + string(template))
	if err != nil {
		return err
	}
	templateConfig := &Config{}
	if err := yaml.Unmarshal(content, templateConfig); err != nil {
		return err
	}
	if err := mergo.Merge(config, templateConfig, mergo.WithOverride); err != nil {
		return err
	}
	config.rawYaml = append(config.rawYaml, string(content[:]))
	return nil
}

func LoadConfig(dir string, configName string, includeTemplates bool, templatesDir string) (*Config, error) {
	config := &Config{
		Name:         configName,
		Boot_Command: DefaultBootCommand,
		Base_Image:   DefaultBaseImage(),
	}
	content, err := os.ReadFile(string(strings.TrimRight(dir, "/") + "/" + config.Name + ".yml"))
	if err != nil {
		return nil, err
	}
	baseConfig := &Config{}

	if err := yaml.Unmarshal(content, baseConfig); err != nil {
		return nil, err
	}

	if includeTemplates {
		for _, t := range baseConfig.Templates {
			if err := config.loadTemplate(templatesDir, t); err != nil {
				return nil, err
			}
		}
	}
	if err := mergo.Merge(config, baseConfig, mergo.WithOverride); err != nil {
		return nil, err
	}
	config.rawYaml = append(config.rawYaml, string(content[:]))
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (config *Config) Yaml() string {
	return strings.Join(config.rawYaml, "_FILE_SEPERATOR_")
}

func (config *Config) WriteDockerCompose(dir string, bakeEnv bool) error {
	if err := config.WriteEnvConfig(dir); err != nil {
		return err
	}
	pupsArgs := "--skip-tags=precompile,migrate,db"
	if err := config.WriteDockerfile(dir, pupsArgs, bakeEnv); err != nil {
		return err
	}
	labels := map[string]string{}
	for k, v := range config.Labels {
		val := strings.ReplaceAll(v, "{{config}}", config.Name)
		labels[k] = val
	}
	env := map[string]string{}
	for k, v := range config.Env {
		val := strings.ReplaceAll(v, "{{config}}", config.Name)
		env[k] = val
	}
	env["CREATE_DB_ON_BOOT"] = "1"
	env["MIGRATE_ON_BOOT"] = "1"

	links := []string{}
	for _, v := range config.Links {
		links = append(links, v.Link.Name+":"+v.Link.Alias)
	}
	volumes := []string{}
	composeVolumes := map[string]*interface{}{}
	for _, v := range config.Volumes {
		volumes = append(volumes, v.Volume.Host+":"+v.Volume.Guest)
		// if this is a docker volume (vs a bind mount), add to global volume list
		matched, _ := regexp.MatchString(`^[A-Za-z]`, v.Volume.Host)
		if matched {
			composeVolumes[v.Volume.Host] = nil
		}
	}
	ports := []string{}
	for _, v := range config.Expose {
		ports = append(ports, v)
	}

	args := []string{}
	for k, _ := range config.Env {
		args = append(args, k)
	}
	compose := &DockerComposeYaml{
		Services: ComposeAppService{
			App: ComposeService{
				Build: ComposeBuild{
					Dockerfile: "./Dockerfile",
					Labels:     labels,
					Shm_Size:   "512m",
					Args:       args,
					No_Cache:   true,
				},
				Environment: env,
				Links:       links,
				Volumes:     volumes,
				Ports:       ports,
			},
		},
		Volumes: composeVolumes,
	}

	var b bytes.Buffer
	encoder := yaml.NewEncoder(&b)
	encoder.SetIndent(2)
	err := encoder.Encode(&compose)
	yaml := b.Bytes()
	if err != nil {
		return errors.New("error marshalling compose file to write docker-compose.yaml")
	}
	if err := os.WriteFile(strings.TrimRight(dir, "/")+"/"+"docker-compose.yaml", yaml, 0666); err != nil {
		return errors.New("error writing compose file docker-compose.yaml")
	}
	return nil
}

func (config *Config) WriteDockerfile(dir string, pupsArgs string, bakeEnv bool) error {
	if err := config.WriteYamlConfig(dir); err != nil {
		return err
	}

	file := strings.TrimRight(dir, "/")+"/"+"Dockerfile"
	if err := os.WriteFile(file, []byte(config.Dockerfile(pupsArgs, bakeEnv)), 0666); err != nil {
		return errors.New("error writing dockerfile Dockerfile " + file)
	}
	return nil
}

func (config *Config) Dockerfile(pupsArgs string, bakeEnv bool) string {
	builder := strings.Builder{}
	builder.WriteString("FROM " + config.Base_Image + "\n")
	builder.WriteString(config.DockerfileArgs() + "\n")
	if bakeEnv {
		builder.WriteString(config.DockerfileEnvs() + "\n")
	}
	builder.WriteString("COPY config.yaml /temp-config.yaml\n")
	builder.WriteString("RUN " +
		"cat /temp-config.yaml | /usr/local/bin/pups " + pupsArgs + " --stdin " +
		"&& rm /temp-config.yaml\n")
	builder.WriteString("CMD " + config.BootCommand())
	return builder.String()
}

func (config *Config) WriteYamlConfig(dir string) error {
	file := strings.TrimRight(dir, "/")+"/config.yaml"
	if err := os.WriteFile(file, []byte(config.Yaml()), 0666); err != nil {
		return errors.New("error writing config file " + file)
	}
	return nil
}

func (config *Config) WriteEnvConfig(dir string) error {
	file := strings.TrimRight(dir, "/")+"/.envrc"
	if err := os.WriteFile(file, []byte(config.ExportEnv()), 0666); err != nil {
		return errors.New("error writing export env " + file)
	}
	return nil
}

func (config *Config) BootCommand() string {
	if config.Boot_Command != "" && config.No_Boot_Command {
		return "/sbin/boot"
	} else {
		return config.Boot_Command
	}
}

func (config *Config) EnvCli() string {
	builder := strings.Builder{}
	for k, _ := range config.Env {
		builder.WriteString(k + "\n")
	}
	return strings.TrimSpace(builder.String())
}

func (config *Config) EnvArray() []string {
	envs := []string{}
	for k, v := range config.Env {
		val := strings.ReplaceAll(v, "{{config}}", config.Name)
		envs = append(envs, k+"="+val)
	}
	return envs
}

func (config *Config) ExportEnv() string {
	builder := strings.Builder{}
	for k, v := range config.Env {
		val := strings.ReplaceAll(v, "{{config}}", config.Name)
		val = strings.ReplaceAll(val, "\"", "\\\"")
		builder.WriteString("export " + k + "=\"" + val + "\"\n")
	}
	return strings.TrimSpace(builder.String())
}

func (config *Config) DockerfileEnvs() string {
	builder := strings.Builder{}
	for k, _ := range config.Env {
		builder.WriteString("ENV " + k + "=${" + k + "}\n")
	}
	return strings.TrimSpace(builder.String())
}

func (config *Config) DockerfileArgs() string {
	builder := strings.Builder{}
	for k, _ := range config.Env {
		builder.WriteString("ARG " + k + "\n")
	}
	return strings.TrimSpace(builder.String())
}

func (config *Config) LabelsCli() string {
	builder := strings.Builder{}
	for k, v := range config.Labels {
		builder.WriteString(" --label " + k + "=" + strings.ReplaceAll(v, "{{config}}", config.Name))
	}
	return strings.TrimSpace(builder.String())
}

func (config *Config) PortsCli() string {
	builder := strings.Builder{}
	for _, p := range config.Expose {
		if strings.Contains(p, ":") {
			builder.WriteString(" -p " + p)
		} else {
			builder.WriteString(" --expose " + p)
		}
	}
	return strings.TrimSpace(builder.String())
}

func (config *Config) VolumesCli() string {
	builder := strings.Builder{}
	for _, v := range config.Volumes {
		builder.WriteString(" -v " + v.Volume.Host + ":" + v.Volume.Guest)
	}
	return strings.TrimSpace(builder.String())
}

func (config *Config) LinksCli() string {
	builder := strings.Builder{}
	for _, l := range config.Links {
		builder.WriteString(" --link " + l.Link.Name + ":" + l.Link.Alias)
	}
	return strings.TrimSpace(builder.String())
}
