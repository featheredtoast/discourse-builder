package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bytes"
	"context"
	ddocker "github.com/discourse/discourse_docker/launcher_go/v2"
	. "github.com/discourse/discourse_docker/launcher_go/v2/test_utils"
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

		var cmdWatch chan utils.ICmdRunner

		var checkStartCmd = func() {
			cmd := GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -q --filter name=test"))
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=test"))
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("-d"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--name test local_discourse/test /sbin/boot"))
		}

		var checkStartCmdWhenStarted = func() {
			cmd := GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -q --filter name=test"))
		}

		var checkStopCmd = func() {
			cmd := GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=test"))
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker stop -t 600 test"))
		}

		var checkStopCmdWhenMissing = func() {
			cmd := GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=test"))
		}

		BeforeEach(func() {
			cmdWatch = make(chan utils.ICmdRunner)
			utils.CmdRunner = CreateNewFakeCmdRunner(cmdWatch)
		})
		AfterEach(func() {
			close(cmdWatch)
		})

		It("should run start commands", func() {
			runner := ddocker.StartCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkStartCmd()
		})

		It("should not run stop commands", func() {
			runner := ddocker.StopCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkStopCmdWhenMissing()
		})

		Context("with a running container", func() {
			BeforeEach(func() {
				//response should be non-empty, indicating a running container
				response := []byte{123}
				utils.CmdRunner = CreateNewFakeCmdRunnerWithOutput(cmdWatch, &response)
			})

			It("should not run start commands", func() {
				runner := ddocker.StartCmd{Config: "test"}
				go runner.Run(cli, &ctx)
				checkStartCmdWhenStarted()
			})

			It("should run stop commands", func() {
				runner := ddocker.StopCmd{Config: "test"}
				go runner.Run(cli, &ctx)
				checkStopCmd()
			})
		})

		It("should keep running during commits, and be post-deploy migration aware when using a web only container", func() {
			runner := ddocker.RebuildCmd{Config: "web_only"}

			go runner.Run(cli, &ctx)

			//initial build
			cmd := GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker build"))

			//migrate, skipping post deployment migrations
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--tags=db,migrate"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env SKIP_POST_DEPLOYMENT_MIGRATIONS=1"))

			// precompile
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--tags=db,precompile"))
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker commit"))
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker rm"))

			// destroying (because we never started from the tests, this will look like a stoppe container, and there is no stop+rm command)
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=web_only"))

			// run expects 2 ps runs on start
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -q --filter name=web_only"))
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker ps -a -q --filter name=web_only"))

			// starting container
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("-d"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("/sbin/boot"))

			// run post-deploy migrations
			cmd = GetLastCommand(cmdWatch)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--tags=db,migrate"))
		})
	})
})
