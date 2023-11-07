package docker

import (
	"context"
	"github.com/discourse/discourse_docker/discourse-builder/config"
	"github.com/discourse/discourse_docker/discourse-builder/utils"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type DockerBuilder struct {
	Config *config.Config
	Ctx    *context.Context
	Stdin  io.Reader
	Dir    string
}

func (r *DockerBuilder) Run() error {
	cmd := exec.CommandContext(*r.Ctx, "docker", "build")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return unix.Kill(-cmd.Process.Pid, unix.SIGINT)
	}
	cmd.Dir = r.Dir
	cmd.Env = r.Config.EnvArray(false)
	cmd.Env = append(cmd.Env, "BUILDKIT_PROGRESS=plain")
	for k, _ := range r.Config.Env {
		cmd.Args = append(cmd.Args, "--build-arg")
		cmd.Args = append(cmd.Args, k)
	}
	cmd.Args = append(cmd.Args, "--no-cache")
	cmd.Args = append(cmd.Args, "--pull")
	cmd.Args = append(cmd.Args, "--force-rm")
	cmd.Args = append(cmd.Args, "-t")
	cmd.Args = append(cmd.Args, utils.BaseImageName+r.Config.Name)
	cmd.Args = append(cmd.Args, "--shm-size=512m")
	cmd.Args = append(cmd.Args, "-f")
	cmd.Args = append(cmd.Args, "-")
	cmd.Args = append(cmd.Args, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = r.Stdin
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	return nil
}

type DockerRunner struct {
	Config      *config.Config
	Ctx         *context.Context
	ExtraEnv    []string
	Rm          bool
	ContainerId string
	Cmd         []string
	Stdin       io.Reader
	SkipPorts   bool
}

func (r *DockerRunner) Run() error {
	cmd := exec.CommandContext(*r.Ctx, "docker", "run")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return unix.Kill(-cmd.Process.Pid, unix.SIGINT)
	}
	cmd.Env = r.Config.EnvArray(true)
	for k, _ := range r.Config.Env {
		cmd.Args = append(cmd.Args, "--env")
		cmd.Args = append(cmd.Args, k)
	}
	for _, e := range r.ExtraEnv {
		cmd.Args = append(cmd.Args, "--env")
		cmd.Args = append(cmd.Args, e)
	}
	for k, v := range r.Config.Labels {
		cmd.Args = append(cmd.Args, "--label")
		cmd.Args = append(cmd.Args, k+"="+v)
	}
	if !r.SkipPorts {
		for _, v := range r.Config.Expose {
			if strings.Contains(v, ":") {
				cmd.Args = append(cmd.Args, "-p")
				cmd.Args = append(cmd.Args, v)
			} else {
				cmd.Args = append(cmd.Args, "--expose")
				cmd.Args = append(cmd.Args, v)
			}
		}
	}
	for _, v := range r.Config.Volumes {
		cmd.Args = append(cmd.Args, "-v")
		cmd.Args = append(cmd.Args, v.Volume.Host+":"+v.Volume.Guest)
	}
	for _, v := range r.Config.Links {
		cmd.Args = append(cmd.Args, "--link")
		cmd.Args = append(cmd.Args, v.Link.Name+":"+v.Link.Alias)
	}
	cmd.Args = append(cmd.Args, "--shm-size=512m")
	if r.Rm {
		cmd.Args = append(cmd.Args, "--rm")
	}
	cmd.Args = append(cmd.Args, "--name")
	cmd.Args = append(cmd.Args, r.ContainerId)
	cmd.Args = append(cmd.Args, "-i")
	cmd.Args = append(cmd.Args, utils.BaseImageName+r.Config.Name)

	for _, c := range r.Cmd {
		cmd.Args = append(cmd.Args, c)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = r.Stdin
	if err := utils.CmdRunner(cmd).Run(); err != nil {
		return err
	}
	return nil
}

type DockerPupsRunner struct {
	Config         *config.Config
	PupsArgs       string
	SavedImageName string
	SkipEmber      bool
	Ctx            *context.Context
	ContainerId    string
}

func (r *DockerPupsRunner) Run() error {
	extraEnv := []string{}
	if r.SkipEmber {
		extraEnv = []string{"SKIP_EMBER_CLI_COMPILE=1"}
	}
	rm := false
	if len(r.SavedImageName) <= 0 {
		rm = true
	}
	commands := []string{"/bin/bash",
		"-c",
		"/usr/local/bin/pups --stdin " + r.PupsArgs}

	runner := DockerRunner{Config: r.Config,
		Ctx:         r.Ctx,
		ExtraEnv:    extraEnv,
		Rm:          rm,
		ContainerId: r.ContainerId,
		Cmd:         commands,
		Stdin:       strings.NewReader(r.Config.Yaml()),
		SkipPorts:   true, //pups runs don't need to expose ports
	}

	if err := runner.Run(); err != nil {
		return err
	}

	if len(r.SavedImageName) > 0 {
		cmd := exec.Command("docker",
			"commit",
			"--change",
			"LABEL org.opencontainers.image.created=\""+time.Now().Format(time.RFC3339)+"\"",
			"--change",
			"CMD "+r.Config.BootCommand(),
			r.ContainerId,
			utils.BaseImageName+r.Config.Name,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := utils.CmdRunner(cmd).Run(); err != nil {
			return err
		}

		//clean up container
		runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd = exec.CommandContext(runCtx, "docker", "rm", "-f", r.ContainerId)
		utils.CmdRunner(cmd).Run()
	}
	return nil
}

func ContainerExists(container string) (bool, error) {
	cmd := exec.Command("docker", "ps", "-a", "-q", "--filter", "name="+container)
	result, err := utils.CmdRunner(cmd).Output()
	if err != nil {
		return false, err
	}
	if len(result) > 0 {
		return true, nil
	}
	return false, nil
}
