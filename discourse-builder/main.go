package main

import (
	"context"
	"fmt"
	"github.com/alecthomas/kong"
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
	ConfDir      string             `short:"c" default:"./containers" help:"Pups config directory."`
	TemplatesDir string             `short:"t" default:"." help:"Parent directory containing a templates/ directory with pups yaml templates."`
	BuildDir     string             `default:"./tmp" help:"Temporary build folder for building images."`
	ForceMkdir   bool               `short:"p" name:"parent-dirs" help:"Create intermediate output directories as required.  If this option is not specified, the full path prefix of each operand must already exist."`
	CliGenerate  CliGenerate        `cmd:"" name:"generate" help:"Generate commands, used to generate Discourse pups configuration for external use."`
	BuildCmd     DockerBuildCmd     `cmd:"" name:"build" help:"Build a base image with no dependencies."`
	ConfigureCmd DockerConfigureCmd `cmd:"" name:"configure" help:"Configure and save an image with all dependencies and environment baked in. Updates themes and precompiles all assets."`
	MigrateCmd   DockerMigrateCmd   `cmd:"" name:"migrate" help:"Run migration tasks from a built image."`
	BootstrapCmd DockerBootstrapCmd `cmd:"" name:"bootstrap" help:"Build, migrate, and configure an image."`

	DestroyCmd DestroyCmd `cmd:"" name:"destroy" help:"Shutdown and destroy container."`
	LogsCmd    LogsCmd    `cmd:"" name:"logs" help:"Print logs for container."`
	CleanupCmd CleanupCmd `cmd:"" name:"cleanup" help:"Cleanup unused containers."`
	EnterCmd   EnterCmd   `cmd:"" name:"enter" help:"Connects to a shell running in the container."`
	RunCmd     RunCmd     `cmd:"" name:"run" help:"Runs the specified command in context of a docker container."`
	StartCmd   StartCmd   `cmd:"" name:"start" help:"Starts container."`
	StopCmd    StopCmd    `cmd:"" name:"stop" help:"Stops container."`
	RestartCmd RestartCmd `cmd:"" name:"restart" help:"Stops then starts container."`
	RebuildCmd RebuildCmd `cmd:"" name:"rebuild" help:"Builds new image, then destroys old container, and starts new container. If PRECOMPILE_ON_BOOT and MIGRATE_ON_BOOT are set in the config, it will start up the container without running migrate and configure steps."`
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
