package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/discourse/discourse_docker/discourse-builder/config"
	"github.com/discourse/discourse_docker/discourse-builder/docker"
	"github.com/discourse/discourse_docker/discourse-builder/utils"
	"os"
	"os/exec"
)

type StartCmd struct {
	Config        string `arg:"" name:"config" help:"config"`
	DryRun        bool   `name:"dry-run" short:"n" help:"print start command only"`
	GenMacAddress bool   `name:"mac-address" negatable:"" help:"assign a mac address"`
	DockerArgs    string `name:"docker-args" help:"Extra arguments to pass when running docker"`
	RunImage      string `name:"run-image" help:"Override the image used for running the container"`
}

func (r *StartCmd) Run(cli *Cli, ctx *context.Context) error {
	_, err := config.LoadConfig(cli.ConfDir, r.Config, true, cli.TemplatesDir)
	if err != nil {
		return errors.New("YAML syntax error. Please check your containers/*.yml config files.")
	}
	//TODO: implement
	return nil
}

type StopCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *StopCmd) Run(cli *Cli, ctx *context.Context) error {
	exists, _ := docker.ContainerExists(r.Config)
	if !exists {
		fmt.Println(r.Config + "was not found")
		return nil
	}
	cmd := exec.CommandContext(*ctx, "docker", "stop", "-t", "600", r.Config)
	fmt.Println(cmd)
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	return nil
}

type RestartCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *RestartCmd) Run(cli *Cli, ctx *context.Context) error {
	start := StartCmd{Config: r.Config}
	stop := StopCmd{Config: r.Config}
	if err := start.Run(cli, ctx); err != nil {
		return err
	}
	if err := stop.Run(cli, ctx); err != nil {
		return err
	}
	return nil
}

type DestroyCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *DestroyCmd) Run(cli *Cli, ctx *context.Context) error {
	exists, _ := docker.ContainerExists(r.Config)
	if !exists {
		fmt.Println(r.Config + "was not found")
		return nil
	}

	cmd := exec.CommandContext(*ctx, "docker", "stop", "-t", "600", r.Config)
	fmt.Println(cmd)
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	cmd = exec.CommandContext(*ctx, "docker", "rm", r.Config)
	fmt.Println(cmd)
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	return nil
}

type EnterCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *EnterCmd) Run(cli *Cli, ctx *context.Context) error {
	cmd := exec.CommandContext(*ctx, "docker", "exec", "-it", r.Config, "/bin/bash", "--login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	return nil
}

type LogsCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *LogsCmd) Run(cli *Cli, ctx *context.Context) error {
	cmd := exec.CommandContext(*ctx, "docker", "logs", r.Config)
	output, err := utils.CmdRunner(cmd).Output()
	if err != nil {
		return err
	}
	fmt.Println(string(output[:]))
	return nil
}

type RebuildCmd struct {
	Config string `arg:"" name:"config" help:"config"`
}

func (r *RebuildCmd) Run(cli *Cli, ctx *context.Context) error {
	//TODO implement
	return nil
}

type CleanupCmd struct{}

func (r *CleanupCmd) Run(cli *Cli, ctx *context.Context) error {
	cmd := exec.CommandContext(*ctx, "docker", "container", "prune", "--filter", "until=1h")
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	cmd = exec.CommandContext(*ctx, "docker", "image", "prune", "--all", "--filter", "until=1h")
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	_, err := os.Stat("/var/discourse/shared/standalone/postgres_data_old")
	if !os.IsNotExist(err) {
		fmt.Println("Old PostgreSQL backup data cluster detected")
		fmt.Println("Would you like to remove it? (y/N)")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		reply := scanner.Text()
		if reply == "y" || reply == "Y" {
			fmt.Println("removing old PostgreSQL data cluster at /var/discourse/shared/standalone/postgres_data_old...")
			os.RemoveAll("/var/discourse/shared/standalone/postgres_data_old")
		} else {
			return errors.New("Cancelled")
		}
	}

	return nil
}
