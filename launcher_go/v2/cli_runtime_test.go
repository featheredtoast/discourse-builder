package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bytes"
	"context"
	ddocker "github.com/discourse/discourse_docker/launcher_go/v2"
	"github.com/discourse/discourse_docker/launcher_go/v2/utils"
	"os"
)

var _ = Describe("Runtime", func() {
	var testDir string
	var out *bytes.Buffer
	var cli *ddocker.Cli
	var ctx context.Context

	BeforeEach(func() {
		utils.DockerPath = "docker"
		out = &bytes.Buffer{}
		utils.Out = out
		testDir, _ = os.MkdirTemp("", "ddocker-test")

		ctx = context.Background()

		cli = &ddocker.Cli{
			ConfDir:      "./test/containers",
			TemplatesDir: "./test",
			BuildDir:     testDir,
		}
	})
	AfterEach(func() {
		os.RemoveAll(testDir)
	})

	Context("When running run commands", func() {

		var CmdCreatorWatcher chan utils.ICmdRunner
		var getLastCommand = func() *FakeCmdRunner {
			icmd := <-CmdCreatorWatcher
			cmd, _ := icmd.(*FakeCmdRunner)
			<-cmd.RunCalls
			return cmd
		}

		var checkStartCmd = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -q --filter name=test"))
			cmd = getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=test"))
			cmd = getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("-d"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--name test local_discourse/test /sbin/boot"))
		}

		var checkStartCmdWhenStarted = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -q --filter name=test"))
		}

		var checkStopCmd = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=test"))
			cmd = getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker stop -t 600 test"))
		}

		var checkStopCmdWhenMissing = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=test"))
		}

		BeforeEach(func() {
			CmdCreatorWatcher = make(chan utils.ICmdRunner)
			utils.CmdRunner = CreateNewFakeCmdRunner(CmdCreatorWatcher)
		})
		AfterEach(func() {
		})

		It("should run start commands", func() {
			runner := ddocker.StartCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkStartCmd()
			close(CmdCreatorWatcher)
		})

		It("should not run stop commands", func() {
			runner := ddocker.StopCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkStopCmdWhenMissing()
			close(CmdCreatorWatcher)
		})

		Context("with a running container", func() {
			BeforeEach(func() {
				//response should be non-empty, indicating a running container
				response := []byte{123}
				utils.CmdRunner = CreateNewFakeCmdRunnerWithOutput(CmdCreatorWatcher, &response)
			})

			It("should not run start commands", func() {
				runner := ddocker.StartCmd{Config: "test"}
				go runner.Run(cli, &ctx)
				checkStartCmdWhenStarted()
				close(CmdCreatorWatcher)
			})

			It("should run stop commands", func() {
				runner := ddocker.StopCmd{Config: "test"}
				go runner.Run(cli, &ctx)
				checkStopCmd()
				close(CmdCreatorWatcher)
			})
		})

	})
})
