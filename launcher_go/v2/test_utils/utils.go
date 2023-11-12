package test_utils

import (
	"github.com/discourse/discourse_docker/launcher_go/v2/utils"
	"os/exec"
	"time"
)

type FakeCmdRunner struct {
	Cmd            *exec.Cmd
	RunCalls       chan int
	OutputResponse *[]byte
}

func (r *FakeCmdRunner) Run() error {
	r.RunCalls <- 1
	return nil
}

func (r *FakeCmdRunner) Output() ([]byte, error) {
	r.RunCalls <- 1
	return *r.OutputResponse, nil
}

// Swap out CmdRunner with a fake instance that also returns created ICmdRunners on a channel
// so tests can inspect commands the moment they're run
func CreateNewFakeCmdRunner(c chan utils.ICmdRunner) func(cmd *exec.Cmd) utils.ICmdRunner {
	return func(cmd *exec.Cmd) utils.ICmdRunner {
		cmdRunner := &FakeCmdRunner{Cmd: cmd,
			RunCalls:       make(chan int),
			OutputResponse: &[]byte{}}
		c <- cmdRunner
		return cmdRunner
	}
}

func CreateNewFakeCmdRunnerWithOutput(c chan utils.ICmdRunner, outputResponse *[]byte) func(cmd *exec.Cmd) utils.ICmdRunner {
	return func(cmd *exec.Cmd) utils.ICmdRunner {
		cmdRunner := &FakeCmdRunner{Cmd: cmd,
			RunCalls:       make(chan int),
			OutputResponse: outputResponse}
		c <- cmdRunner
		return cmdRunner
	}
}

func GetLastCommand(cmdCreatorWatcher chan utils.ICmdRunner) *FakeCmdRunner {
	select {
	case icmd := <-cmdCreatorWatcher:
		cmd, _ := icmd.(*FakeCmdRunner)
		<-cmd.RunCalls
		return cmd
	case <-time.After(time.Second):
		panic("no command!")
	}
}
