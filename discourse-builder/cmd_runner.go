package main

import (
	"os/exec"
)

type ICmdRunner interface {
	Run() error
}

type ExecCmdRunner struct {
	Cmd *exec.Cmd
}

func (r *ExecCmdRunner) Run() error {
	return r.Cmd.Run()
}

func NewExecCmdRunner(cmd *exec.Cmd) ICmdRunner {
	return &ExecCmdRunner{Cmd: cmd}
}

var CmdRunner = NewExecCmdRunner
