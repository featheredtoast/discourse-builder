package main

import (
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/discourse/discourse_docker/launcher_go/v2/utils"
	"github.com/posener/complete"
	"github.com/willabides/kongplete"
	"golang.org/x/sys/unix"
	"os"
	"os/signal"
)

type Cli struct {
	Version      kong.VersionFlag   `help:"Show version."`
	ConfDir      string             `default:"./containers" help:"Discourse pups config directory." predictor:"dir"`
	TemplatesDir string             `default:"." help:"Home project directory containing a templates/ directory which in turn contains pups yaml templates." predictor:"dir"`
	BuildDir     string             `default:"./tmp" help:"Temporary build folder for building images." predictor:"dir"`
	DockerCompose DockerComposeCmd `cmd:"" name:"compose" help:"Create docker compose setup in the output {output-directory}/{config}/. The builder generates a docker-compose.yaml, Dockerfile, config.yaml, and an env file for you to source .envrc. Run with 'source .envrc; docker compose up'."`
	ConcourseJob  ConcourseJobCmd  `cmd:"" name:"concourse-job" help:"Print concourse job config"`

	InstallCompletions kongplete.InstallCompletions `cmd:"" aliases:"sh" help:"Print shell autocompletions. Add output to dotfiles, or 'source <(./launcher-extras sh)'."`
}

func main() {
	cli := Cli{}
	runCtx, cancel := context.WithCancel(context.Background())

	// pre parse to get config dir for prediction of conf dir
	confFiles := utils.FindConfigNames()

	parser := kong.Must(&cli, kong.UsageOnError(), kong.Bind(&runCtx), kong.Vars{"version": "v1.0.0"})

	// Run kongplete.Complete to handle completion requests
	kongplete.Complete(parser,
		kongplete.WithPredictor("config", complete.PredictSet(confFiles...)),
		kongplete.WithPredictor("file", complete.PredictFiles("*")),
		kongplete.WithPredictor("dir", complete.PredictDirs("*")),
	)

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, unix.SIGTERM)
	signal.Notify(sigChan, unix.SIGINT)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-sigChan:
			fmt.Fprintln(utils.Out, "Command interrupted")
			cancel()
		case <-done:
		}
	}()
	err = ctx.Run()
	ctx.FatalIfErrorf(err)
}
