package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/discourse/discourse_docker/launcher_go/v2/config"
	"github.com/discourse/discourse_docker/launcher_go/v2/utils"

	"gopkg.in/yaml.v3"
)

type DockerComposeYaml struct {
	Services ComposeAppService
	Volumes  map[string]*interface{}
}
type ComposeAppService struct {
	App ComposeService
}
type ComposeService struct {
	Image       string
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

func WriteDockerCompose(config config.Config, dir string, bakeEnv bool) error {
	if err := WriteEnvConfig(config, dir); err != nil {
		return err
	}
	pupsArgs := "--skip-tags=precompile,migrate,db"
	if err := WriteDockerfile(config, dir, pupsArgs, bakeEnv); err != nil {
		return err
	}
	labels := map[string]string{}
	for k, v := range config.Labels {
		labels[k] = v
	}
	env := map[string]string{}
	for k, v := range config.Env {
		env[k] = v
	}
	env["CREATE_DB_ON_BOOT"] = "1"
	env["MIGRATE_ON_BOOT"] = "1"

	links := []string{}
	for _, v := range config.Links {
		links = append(links, v.Link.Name+":"+v.Link.Alias)
	}
	slices.Sort(links)
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
	slices.Sort(volumes)
	ports := []string{}
	for _, v := range config.Expose {
		ports = append(ports, v)
	}
	slices.Sort(ports)

	args := []string{}
	for k, _ := range config.Env {
		args = append(args, k)
	}
	slices.Sort(args)
	compose := &DockerComposeYaml{
		Services: ComposeAppService{
			App: ComposeService{
				Image: utils.DefaultNamespace + "/" + config.Name,
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
	if err := os.WriteFile(strings.TrimRight(dir, "/")+"/"+"docker-compose.yaml", yaml, 0660); err != nil {
		return errors.New("error writing compose file docker-compose.yaml")
	}
	return nil
}

func WriteDockerfile(config config.Config, dir string, pupsArgs string, bakeEnv bool) error {
	if err := config.WriteYamlConfig(dir); err != nil {
		return err
	}

	file := strings.TrimRight(dir, "/") + "/" + "Dockerfile"
	if err := os.WriteFile(file, []byte(config.Dockerfile(pupsArgs, bakeEnv)), 0660); err != nil {
		return errors.New("error writing dockerfile Dockerfile " + file)
	}
	return nil
}

func WriteEnvConfig(config config.Config, dir string) error {
	file := strings.TrimRight(dir, "/") + "/.envrc"
	if err := os.WriteFile(file, []byte(ExportEnv(config)), 0660); err != nil {
		return errors.New("error writing export env " + file)
	}
	return nil
}

func ExportEnv(config config.Config) string {
	builder := []string{}
	for k, v := range config.Env {
		val := strings.ReplaceAll(v, "\\", "\\\\")
		val = strings.ReplaceAll(val, "\"", "\\\"")
		builder = append(builder, "export "+k+"=\""+val+"\"")
	}
	slices.Sort(builder)
	return strings.Join(builder, "\n")
}

type DockerComposeCmd struct {
	OutputDir string `name:"output dir" default:"./compose" short:"o" help:"Output dir for docker compose files." predictor:"dir"`
	BakeEnv   bool   `short:"e" help:"Bake in the configured environment to image after build."`

	Config string `arg:"" name:"config" help:"config" predictor:"config"`
}

func (r *DockerComposeCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	dir := r.OutputDir + "/" + r.Config
	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := WriteDockerCompose(*config, dir, r.BakeEnv); err != nil {
		return err
	}
	return nil
}
