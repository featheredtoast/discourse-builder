package main

import (
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"os/signal"
)

var Out io.Writer = os.Stdout

// TODO file permissions on output probably better set 640
// TODO dry run start output now needs to be substituted with env so it can be run outside? right now env is --env ENV rather than --env ENV=VAL
type Cli struct {
	ConfDir      string `short:"c" default:"./containers" help:"pups config directory"`
	TemplatesDir string `short:"t" default:"." help:"parent directory containing a templates/ directory with pups yaml templates"`
	// TODO: how do we handle output dir? we have a temp dir and output dir -- tmp for running and building stuff, output for generate files
	OutputDir    string             `short:"o" default:"./tmp" help:"parent output folder"`
	ContainerId  string             `hidden:"" optional:""`
	ForceMkdir   bool               `short:"p" name:"parent-dirs" help:"Create intermediate output directories as required.  If this option is not specified, the full path prefix of each operand must already exist."`
	CliGenerate  CliGenerate        `cmd:"" name:"generate" help:"generate commands"`
	BuildCmd     DockerBuildCmd     `cmd:"" name:"build" help:"Build a base image with no dependencies."`
	ConfigureCmd DockerConfigureCmd `cmd:"" name:"configure" help:"Configure and save an image with all dependencies and environment baked in. Updates themes and precompiles all assets."`
	MigrateCmd   DockerMigrateCmd   `cmd:"" name:"migrate" help:"Run migration tasks on an image."`
	BootstrapCmd DockerBootstrapCmd `cmd:"" name:"bootstrap" help:"Build, migrate, and configure an image"`

	DestroyCmd DestroyCmd `cmd:"" name:"destroy" help:"shutdown and destroy container"`
	LogsCmd    LogsCmd    `cmd:"" name:"logs" help:"print logs for container"`
	CleanupCmd CleanupCmd `cmd:"" name:"cleanup" help:"cleanup containers"`
	EnterCmd   EnterCmd   `cmd:"" name:"enter" help:"enter container"`
	RunCmd     RunCmd     `cmd:"" name:"run" help:"runs command in docker container"`
	StartCmd   StartCmd   `cmd:"" name:"start" help:"starts container"`
	StopCmd    StopCmd    `cmd:"" name:"stop" help:"stops container"`
	RestartCmd RestartCmd `cmd:"" name:"restart" help:"restarts container"`
	RebuildCmd RebuildCmd `cmd:"" name:"rebuild" help:"rebuilds container"`
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
