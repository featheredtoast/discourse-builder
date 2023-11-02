package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/discourse/discourse-builder/config"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
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
	if err := os.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := config.WriteYamlConfig(dir); err != nil {
		return err
	}

	cmd := exec.CommandContext(*ctx, "docker", "build")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return unix.Kill(-cmd.Process.Pid, unix.SIGINT)
	}
	cmd.Dir = dir
	cmd.Env = config.EnvArray()
	cmd.Env = append(cmd.Env, "BUILDKIT_PROGRESS=plain")
	for k, _ := range config.Env {
		cmd.Args = append(cmd.Args, "--build-arg")
		cmd.Args = append(cmd.Args, k)
	}
	cmd.Args = append(cmd.Args, "--force-rm")
	cmd.Args = append(cmd.Args, "-t")
	cmd.Args = append(cmd.Args, "local_discourse/"+config.Name)
	cmd.Args = append(cmd.Args, "--shm-size=512m")
	cmd.Args = append(cmd.Args, "-f")
	cmd.Args = append(cmd.Args, "-")
	cmd.Args = append(cmd.Args, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	pupsArgs := "--skip-tags=precompile,migrate,db"
	cmd.Stdin = strings.NewReader(config.Dockerfile("./"+config.Name+".config.yaml", pupsArgs, r.BakeEnv))
	if err := CmdRunner(cmd).Run(); err != nil {
		return err
	}
	cleaner := CleanCmd{Config: r.Config}
	cleaner.Run(cli)

	return nil
}

type DockerPupsCmd struct {
	Config         string `arg:"" name:"config" help:"configuration"`
	PupsArgs       string `short:"p" name:"pups-args" help:"Additional pups args to run with."`
	SavedImageName string `short:"s" name:"saved-image" help:"Name of the resulting docker image. Image will only be committed if set."`
	SkipEmber      bool   `name:"skip-ember" help:"Skip ember compile"`
}

func (r *DockerPupsCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}

	containerId := "discourse-build-" + uuid.NewString()
	cmd := exec.CommandContext(*ctx, "docker", "run")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return unix.Kill(-cmd.Process.Pid, unix.SIGINT)
	}
	cmd.Env = config.EnvArray()
	cmd.Env = append(cmd.Env, "BUILDKIT_PROGRESS=plain")
	for k, _ := range config.Env {
		cmd.Args = append(cmd.Args, "-e")
		cmd.Args = append(cmd.Args, k)
	}
	if r.SkipEmber {
		cmd.Args = append(cmd.Args, "-e")
		cmd.Args = append(cmd.Args, "SKIP_EMBER_CLI_COMPILE=1")
	}
	for k, v := range config.Labels {
		cmd.Args = append(cmd.Args, "--label")
		cmd.Args = append(cmd.Args, k+"="+strings.ReplaceAll(v, "{{config}}", config.Name))
	}
	for _, v := range config.Expose {
		if strings.Contains(v, ":") {
			cmd.Args = append(cmd.Args, "-p")
			cmd.Args = append(cmd.Args, v)
		} else {
			cmd.Args = append(cmd.Args, "--expose")
			cmd.Args = append(cmd.Args, v)
		}
	}
	for _, v := range config.Volumes {
		cmd.Args = append(cmd.Args, "-v")
		cmd.Args = append(cmd.Args, v.Volume.Host+":"+v.Volume.Guest)
	}
	for _, v := range config.Links {
		cmd.Args = append(cmd.Args, "--link")
		cmd.Args = append(cmd.Args, v.Link.Name+":"+v.Link.Alias)
	}
	cmd.Args = append(cmd.Args, "--rm")
	cmd.Args = append(cmd.Args, "--shm-size=512m")
	cmd.Args = append(cmd.Args, "--name")
	cmd.Args = append(cmd.Args, containerId)
	cmd.Args = append(cmd.Args, "-i")
	cmd.Args = append(cmd.Args, "local_discourse/"+config.Name)
	cmd.Args = append(cmd.Args, "/bin/bash")
	cmd.Args = append(cmd.Args, "-c")
	cmd.Args = append(cmd.Args, "/usr/local/bin/pups --stdin "+r.PupsArgs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(config.Yaml())
	if err := CmdRunner(cmd).Run(); err != nil {
		return err
	}
	cleaner := CleanCmd{Config: r.Config}
	cleaner.Run(cli)

	if len(r.SavedImageName) > 0 {
		cmd := exec.Command("docker",
			"commit",
			"--change",
			"LABEL org.opencontainers.image.created=\""+time.Now().Format(time.RFC3339)+"\"",
			containerId,
			"local_discourse/"+config.Name,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = strings.NewReader(config.Yaml())
		CmdRunner(cmd).Run()
	}

	return nil
}

type DockerConfigureCmd struct {
	Config string `arg:"" name:"config" help:"configuration"`
}

func (r *DockerConfigureCmd) Run(cli *Cli, ctx *context.Context) error {
	pups := DockerPupsCmd{
		Config:         r.Config,
		PupsArgs:       "--tags=db,precompile",
		SavedImageName: "local_discourse/" + r.Config,
	}
	return pups.Run(cli, ctx)
}

type DockerMigrateCmd struct {
	Config string `arg:"" name:"config" help:"configuration"`
}

func (r *DockerMigrateCmd) Run(cli *Cli, ctx *context.Context) error {
	pups := DockerPupsCmd{
		Config:    r.Config,
		PupsArgs:  "--tags=db,migrate",
		SkipEmber: true,
	}
	return pups.Run(cli, ctx)
}

type DockerComposeCmd struct {
	BakeEnv bool `short:"e" help:"Bake in the configured environment to image after build. Requires a 'source {config}.env' before running."`

	Config string `arg:"" name:"config" help:"configuration"`
}

func (r *DockerComposeCmd) Run(cli *Cli, ctx *context.Context) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	dir := cli.OutputDir + "/" + r.Config
	if err := os.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := config.WriteDockerCompose(dir, r.BakeEnv); err != nil {
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
	os.Remove(dir + "/" + r.Config + ".config.yaml")
	os.Remove(dir + "/" + r.Config + ".env")
	os.Remove(dir + "/" + "Dockerfile." + r.Config)
	if err := os.Remove(dir); err != nil {
		return err
	}
	return nil
}

type RawYamlCmd struct {
	Config string `arg:"" name:"config" help:"configuration"`
}

func (r *RawYamlCmd) Run(cli *Cli) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	fmt.Fprint(Out, config.Yaml())
	return nil
}

type ParseCmd struct {
	Type       string `help:"type of docker run argument to print. One of: ports, env, labels, args, volumes, links, run-image, boot-command, base-image, update-pups"`
	DockerArgs string `default:"" help:"Extra arguments to pass when running docker."`
	Config     string `arg:"" name:"config" help:"configuration"`
}

func (r *ParseCmd) Run(cli *Cli) error {
	config, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	switch r.Type {
	case "ports":
		fmt.Fprint(Out, config.PortsCli())
	case "env":
		fmt.Fprint(Out, config.EnvCli())
	case "labels":
		fmt.Fprint(Out, config.LabelsCli())
	case "args":
		fmt.Fprint(Out, r.DockerArgs+" "+config.Docker_Args)
	case "volumes":
		fmt.Fprint(Out, config.VolumesCli())
	case "links":
		fmt.Fprint(Out, config.LinksCli())
	case "run-image":
		fmt.Fprint(Out, config.Run_Image)
	case "boot-command":
		if config.Boot_Command != "" && config.No_Boot_Command {
			fmt.Fprint(Out, "/sbin/boot")
		} else {
			fmt.Fprint(Out, config.Boot_Command)
		}
	case "base-image":
		fmt.Fprint(Out, config.Base_Image)
	case "update-pups":
		fmt.Fprint(Out, config.Update_Pups)
	default:
		return errors.New("Unknown parse type. Required -t one of: ports, env, labels, args, volumes, links, run-image, boot-command, base-image, update-pups")
	}
	return nil
}

type Cli struct {
	ConfDir       string             `short:"c" default:"./containers" help:"pups config directory"`
	TemplatesDir  string             `short:"t" default:"." help:"parent directory containing a templates/ directory with pups yaml templates"`
	OutputDir     string             `short:"o" default:"./tmp" help:"parent output folder"`
	Mkdir         bool               `short:"f" help:"force-create parent output folder if not exists"` //TODO: actually do this flag
	DockerCompose DockerComposeCmd   `cmd:"" name:"docker-compose" help:"Create docker compose setup"`
	RawYaml       RawYamlCmd         `cmd:"" name:"raw-yaml" help:"Print raw config, concatenated in pups format"`
	ParseConfig   ParseCmd           `cmd:"" name:"parse" help:"Parse and print config for docker"`
	BuildCmd      DockerBuildCmd     `cmd:"" name:"build" help:"Build a base image with no dependencies."`
	ConfigureCmd  DockerConfigureCmd `cmd:"" name:"configure" help:"Configure and save an image with all dependencies and environment baked in. Updates themes and precompiles all assets."`
	MigrateCmd    DockerMigrateCmd   `cmd:"" name:"migrate" help:"Run migration tasks on an image."`
	Clean         CleanCmd           `cmd:"" name:"clean" help:"clean generated files for config"`
}

func main() {
	cli := Cli{}
	runCtx, cancel := context.WithCancel(context.Background())
	ctx := kong.Parse(&cli, kong.UsageOnError(), kong.Bind(&runCtx))
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, unix.SIGTERM)
	signal.Notify(sigChan, unix.SIGINT)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-sigChan:
			fmt.Fprintln(Out, "Command interrupted")
			cancel()
		case <-done:
		}
	}()
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
