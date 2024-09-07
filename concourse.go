package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/discourse/discourse_docker/launcher_go/v2/config"
	"github.com/discourse/discourse_docker/launcher_go/v2/utils"

	"gopkg.in/yaml.v3"
)

type ConcourseRepo struct {
	Repository string
}
type ConcourseImageResource struct {
	Type   string
	Source ConcourseRepo
}
type ConcourseIo struct {
	Name string
}
type ConcourseRun struct {
	Path string
}
type ConcourseTask struct {
	Params        yaml.Node
	Platform      string
	ImageResource ConcourseImageResource `yaml:"image_resource"`
	Inputs        []ConcourseIo
	Outputs       []ConcourseIo
	Run           ConcourseRun
}

type ConcourseConfig struct {
	FromNamespace     string `yaml:"from_namespace"`
	FromTag           string `yaml:"from_tag"`
	Dockerfile    string
	ConcourseTask string `yaml:"concourse_task"`
	Config        string
}

func getConcourseTask(config config.Config) string {
	content := []*yaml.Node{}
	for k, v := range config.Env {
		key := yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "BUILD_ARG_" + k,
		}
		val := yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: v,
		}
		content = append(content, &key)
		content = append(content, &val)
	}
	params := yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     "!!map",
		Content: content,
	}
	concourseTask := &ConcourseTask{
		Platform: "linux",
		Params:   params,
		ImageResource: ConcourseImageResource{
			Type:   "registry-image",
			Source: ConcourseRepo{Repository: "concourse/oci-build-task"},
		},
		Inputs:  []ConcourseIo{ConcourseIo{Name: "docker-config"}, ConcourseIo{Name: "docker-from-image"}},
		Outputs: []ConcourseIo{ConcourseIo{Name: "image"}},
		Run:     ConcourseRun{Path: "build"},
	}
	var b bytes.Buffer
	encoder := yaml.NewEncoder(&b)
	encoder.SetIndent(2)
	encoder.Encode(&concourseTask)
	yaml := b.Bytes()
	return string(yaml)
}

// generates a yaml file containing:
// dockerfile, concoursetask, config
// which may be used in a static concourse resource
// to generate build jobs
func GenConcourseConfig(config config.Config) string {

	const defaultBaseImage = "discourse/base:2.0.20240825-0027"
	parts := strings.Split(defaultBaseImage, ":")
	namespace := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	concourseConfig := &ConcourseConfig{
		FromNamespace:     namespace,
		FromTag:           tag,
		Dockerfile:    config.Dockerfile("--skip-tags=precompile,migrate,db", false),
		ConcourseTask: getConcourseTask(config),
		Config:        config.Yaml(),
	}

	var b bytes.Buffer
	encoder := yaml.NewEncoder(&b)
	encoder.SetIndent(2)
	encoder.Encode(&concourseConfig)
	yaml := b.Bytes()
	return string(yaml)
}

func WriteConcourseConfig(config config.Config, file string) error {
	if err := os.WriteFile(file, []byte(GenConcourseConfig(config)), 0660); err != nil {
		return errors.New("error writing concourse job config " + file)
	}
	return nil
}

type ConcourseJobCmd struct {
	Output string `help:"write concourse job to output file"`
	Config string `arg:"" name:"config" help:"config" predictor:"config"`
}

func (r *ConcourseJobCmd) Run(cli *Cli) error {
	loadedConfig, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	if r.Output == "" {
		fmt.Fprint(utils.Out, GenConcourseConfig(*loadedConfig))
	} else {
		WriteConcourseConfig(*loadedConfig, r.Output)
	}
	return nil
}
