package main

import (
	"context"
	"errors"
	"github.com/discourse/discourse_docker/discourse-builder/config"
	"github.com/discourse/discourse_docker/discourse-builder/docker"
	"github.com/discourse/discourse_docker/discourse-builder/utils"
	"os"
	"strings"
)

/*
 * build
 * migrate
 * configure
 * bootstrap
 */
type DockerBuildCmd struct {
	BakeEnv bool `short:"e" help:"Bake in the configured environment to image after build."`

	Config string `arg:"" name:"config" help:"configuration"`
}

func (r *DockerBuildCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}

	dir := cli.BuildDir + "/" + r.Config
	if cli.ForceMkdir {
		if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		if err := os.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
			return err
		}
	}
	if err := config.WriteYamlConfig(dir); err != nil {
		return err
	}

	pupsArgs := "--skip-tags=precompile,migrate,db"
	builder := docker.DockerBuilder{
		Config: config,
		Ctx:    ctx,
		Stdin:  strings.NewReader(config.Dockerfile(pupsArgs, r.BakeEnv)),
		Dir:    dir,
	}
	if err := builder.Run(); err != nil {
		return err
	}
	cleaner := CleanCmd{Config: r.Config}
	cleaner.Run(cli)

	return nil
}

type DockerConfigureCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *DockerConfigureCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	pups := docker.DockerPupsRunner{
		Config:         config,
		PupsArgs:       "--tags=db,precompile",
		SavedImageName: utils.BaseImageName + r.Config,
		SkipEmber:      true,
		Ctx:            ctx,
		ContainerId:    cli.ContainerId,
	}
	return pups.Run()
}

type DockerMigrateCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *DockerMigrateCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	pups := docker.DockerPupsRunner{
		Config:      config,
		PupsArgs:    "--tags=db,migrate",
		SkipEmber:   true,
		Ctx:         ctx,
		ContainerId: cli.ContainerId,
	}
	return pups.Run()
}

type DockerBootstrapCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *DockerBootstrapCmd) Run(cli *Cli, ctx *context.Context) error {
	buildStep := DockerBuildCmd{Config: r.Config, BakeEnv: false}
	migrateStep := DockerMigrateCmd{Config: r.Config}
	configureStep := DockerConfigureCmd{Config: r.Config}
	if err := buildStep.Run(cli, ctx); err != nil {
		return err
	}
	if err := migrateStep.Run(cli, ctx); err != nil {
		return err
	}
	if err := configureStep.Run(cli, ctx); err != nil {
		return err
	}
	return nil
}

type CleanCmd struct {
	Config string `arg:"" name:"config" help:"config to clean"`
}

func (r *CleanCmd) Run(cli *Cli) error {
	dir := cli.BuildDir + "/" + r.Config
	os.Remove(dir + "/docker-compose.yaml")
	os.Remove(dir + "/config.yaml")
	os.Remove(dir + "/.envrc")
	os.Remove(dir + "/" + "Dockerfile")
	if err := os.Remove(dir); err != nil {
		return err
	}
	return nil
}
