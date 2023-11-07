package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/discourse/discourse_docker/discourse-builder/config"
	"os"
)

/*
 * raw-yaml
 * compose
 * args (args, run-image, boot-command, hostname)
 */

type CliGenerate struct {
	DockerCompose DockerComposeCmd `cmd:"" name:"docker-compose" help:"Create docker compose setup. The builder also generates an env file for you to source {conf}.env to handle multiline environment vars before running docker compose build"`
	DockerArgs    DockerArgsCmd    `cmd:"" name:"docker-args" help:"Generate docker run args"`
	RawYaml RawYamlCmd `cmd:"" name:"raw-yaml" help:"Print raw config, concatenated in pups format"`
}

type RawYamlCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *RawYamlCmd) Run(cli *Cli) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	fmt.Fprint(Out, config.Yaml())
	return nil
}


type DockerComposeCmd struct {
	OutputDir string `name:"output dir" default:"./compose" short:"o" help:"output dir for the environment"`

	Config string `arg:"" name:"config" help:"config"`
}

func (r *DockerComposeCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	dir := r.OutputDir + "/" + r.Config
	if cli.ForceMkdir {
		if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		if err := os.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
			return err
		}
	}
	if err := config.WriteDockerCompose(dir, false); err != nil {
		return err
	}
	return nil
}

type DockerArgsCmd struct {
	Config       string `arg:"" name:"config" help:"config"`
	Type         string `default:"args" enum:"args,run-image,boot-command,hostname" help:"the type of run arg - args, run-image, boot-command, hostname"`
	IncludePorts bool   `default:"true" name:"include-ports" negatable:"" help:"include ports in run args"`
}

func (r *DockerArgsCmd) Run(cli *Cli) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	switch r.Type {
	case "args":
		fmt.Fprint(Out, config.DockerArgsCli(r.IncludePorts))
	case "run-image":
		fmt.Fprint(Out, config.RunImage())
	case "boot-command":
		fmt.Fprint(Out, config.BootCommand())
	case "hostname":
		fmt.Fprint(Out, config.DockerHostname(""))
	default:
		return errors.New("unknown docker args type")
	}
	return nil
}
