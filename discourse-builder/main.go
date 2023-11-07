package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/discourse/discourse_docker/discourse-builder/config"
	"github.com/discourse/discourse_docker/discourse-builder/utils"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

var Out io.Writer = os.Stdout

type DockerBuildCmd struct {
	BakeEnv bool `short:"e" help:"Bake in the configured environment to image after build."`

	Config string `arg:"" name:"config" help:"configuration"`
}

func (r *DockerBuildCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}

	dir := cli.OutputDir + "/" + r.Config
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
	builder := DockerBuilder{
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
	pups := DockerPupsRunner{
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
	pups := DockerPupsRunner{
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
	dir := cli.OutputDir + "/" + r.Config
	os.Remove(dir + "/docker-compose.yaml")
	os.Remove(dir + "/config.yaml")
	os.Remove(dir + "/.envrc")
	os.Remove(dir + "/" + "Dockerfile")
	if err := os.Remove(dir); err != nil {
		return err
	}
	return nil
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

type Cli struct {
	ConfDir      string             `short:"c" default:"./containers" help:"pups config directory"`
	TemplatesDir string             `short:"t" default:"." help:"parent directory containing a templates/ directory with pups yaml templates"`
	OutputDir    string             `short:"o" default:"./tmp" help:"parent output folder"`
	ContainerId  string             `hidden:"" optional:""`
	ForceMkdir   bool               `short:"p" name:"parent-dirs" help:"Create intermediate output directories as required.  If this option is not specified, the full path prefix of each operand must already exist."`
	CliGenerate  CliGenerate        `cmd:"" name:"generate" help:"generate commands"`
	RawYaml      RawYamlCmd         `cmd:"" name:"raw-yaml" help:"Print raw config, concatenated in pups format"`
	BuildCmd     DockerBuildCmd     `cmd:"" name:"build" help:"Build a base image with no dependencies."`
	ConfigureCmd DockerConfigureCmd `cmd:"" name:"configure" help:"Configure and save an image with all dependencies and environment baked in. Updates themes and precompiles all assets."`
	MigrateCmd   DockerMigrateCmd   `cmd:"" name:"migrate" help:"Run migration tasks on an image."`
	BootstrapCmd DockerBootstrapCmd `cmd:"" name:"bootstrap" help:"Build, migrate, and configure an image"`
	Clean        CleanCmd           `cmd:"" name:"clean" help:"clean generated files for config"`
}

func main() {
	cli := Cli{}
	runCtx, cancel := context.WithCancel(context.Background())
	ctx := kong.Parse(&cli, kong.UsageOnError(), kong.Bind(&runCtx))
	if cli.ContainerId == "" {
		cli.ContainerId = "discourse-build-" + uuid.NewString()
	}
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, unix.SIGTERM)
	signal.Notify(sigChan, unix.SIGINT)
	done := make(chan struct{})
	defer close(done)
	go func(containerId string) {
		select {
		case <-sigChan:
			fmt.Fprintln(Out, "Command interrupted")
			cancel()
		case <-done:
		}
	}(cli.ContainerId)
	err := ctx.Run()
	if err == nil {
		return
	}
	if exiterr, ok := err.(*exec.ExitError); ok {
		// Magic exit code that indicates a retry
		if exiterr.ExitCode() == 77 {
			os.Exit(77)
		} else {
			ctx.Fatalf(
				"run failed with exit code %v\n"+
					"** FAILED TO BOOTSTRAP ** please scroll up and look for earlier error messages, there may be more than one.\n"+
					"./discourse-doctor may help diagnose the problem.", exiterr.ExitCode())
		}
	} else {
		ctx.FatalIfErrorf(err)
	}
}
