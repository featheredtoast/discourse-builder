package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/discourse/discourse_docker/launcher_go/v2/utils"
	"os/exec"
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

var _ = Describe("Main", func() {
	It("exists", func() {
		Expect(true).To(BeTrue())
	})
})
