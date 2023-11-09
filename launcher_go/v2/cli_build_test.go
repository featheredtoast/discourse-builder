package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bytes"
	"context"
	ddocker "github.com/discourse/discourse_docker/launcher_go/v2"
	"github.com/discourse/discourse_docker/launcher_go/v2/utils"
	"io"
	"os"
	"strings"
)

var _ = Describe("Build", func() {
	var testDir string
	var out *bytes.Buffer
	var cli *ddocker.Cli
	var ctx context.Context

	BeforeEach(func() {
		utils.DockerPath = "docker"
		out = &bytes.Buffer{}
		ddocker.Out = out
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

	Context("When running build commands", func() {

		var CmdCreatorWatcher chan utils.ICmdRunner
		var getLastCommand = func() *FakeCmdRunner {
			icmd := <-CmdCreatorWatcher
			cmd, _ := icmd.(*FakeCmdRunner)
			<-cmd.RunCalls
			return cmd
		}

		var checkBuildCmd = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker build"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--build-arg DISCOURSE_DEVELOPER_EMAILS"))
			Expect(cmd.Cmd.Dir).To(Equal(testDir + "/test"))

			//db password is ignored
			Expect(cmd.Cmd.Env).ToNot(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))
			Expect(cmd.Cmd.Env).ToNot(ContainElement("DISCOURSEDB_SOCKET="))
			buf := new(strings.Builder)
			io.Copy(buf, cmd.Cmd.Stdin)
			// docker build's stdin is a dockerfile
			Expect(buf.String()).To(ContainSubstring("COPY config.yaml /temp-config.yaml"))
			Expect(buf.String()).To(ContainSubstring("--skip-tags=precompile,migrate,db"))
			Expect(buf.String()).ToNot(ContainSubstring("SKIP_EMBER_CLI_COMPILE=1"))
		}

		var checkMigrateCmd = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env DISCOURSE_DEVELOPER_EMAILS"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env SKIP_EMBER_CLI_COMPILE=1"))
			// no commit after, we expect an --rm as the container isn't needed after it is stopped
			Expect(cmd.Cmd.String()).To(ContainSubstring("--rm"))
			Expect(cmd.Cmd.Env).To(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))
			buf := new(strings.Builder)
			io.Copy(buf, cmd.Cmd.Stdin)
			// docker run's stdin is a pups config
			Expect(buf.String()).To(ContainSubstring("path: /etc/service/nginx/run"))
		}

		var checkConfigureCmd = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env DISCOURSE_DEVELOPER_EMAILS"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env SKIP_EMBER_CLI_COMPILE=1"))
			// we commit, we need the container to stick around after it is stopped.
			Expect(cmd.Cmd.String()).ToNot(ContainSubstring("--rm"))

			// we don't expose ports on configure command
			Expect(cmd.Cmd.String()).ToNot(ContainSubstring("-p 80"))
			Expect(cmd.Cmd.Env).To(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))
			buf := new(strings.Builder)
			io.Copy(buf, cmd.Cmd.Stdin)
			// docker run's stdin is a pups config
			Expect(buf.String()).To(ContainSubstring("path: /etc/service/nginx/run"))

			// commit on configure
			cmd = getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker commit"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--change CMD [\"/sbin/boot\"]"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("discourse-build"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("local_discourse/test"))
			Expect(cmd.Cmd.Env).ToNot(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))

			// configure also cleans up
			cmd = getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker rm -f discourse-build-"))
		}

		BeforeEach(func() {
			CmdCreatorWatcher = make(chan utils.ICmdRunner)
			utils.CmdRunner = CreateNewFakeCmdRunner(CmdCreatorWatcher)
		})
		AfterEach(func() {
		})

		It("Should run docker build with correct arguments", func() {
			runner := ddocker.DockerBuildCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkBuildCmd()
		})

		It("Should run docker migrate with correct arguments", func() {
			runner := ddocker.DockerMigrateCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkMigrateCmd()
		})

		It("Should allow skip post deployment migrations", func() {
			runner := ddocker.DockerMigrateCmd{Config: "test", SkipPostDeploymentMigrations: true}
			go runner.Run(cli, &ctx)
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env DISCOURSE_DEVELOPER_EMAILS"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env SKIP_POST_DEPLOYMENT_MIGRATIONS=1"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--env SKIP_EMBER_CLI_COMPILE=1"))
			// no commit after, we expect an --rm as the container isn't needed after it is stopped
			Expect(cmd.Cmd.String()).To(ContainSubstring("--rm"))
			Expect(cmd.Cmd.Env).To(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))
			buf := new(strings.Builder)
			io.Copy(buf, cmd.Cmd.Stdin)
			// docker run's stdin is a pups config
			Expect(buf.String()).To(ContainSubstring("path: /etc/service/nginx/run"))
		})

		It("Should run docker run followed by docker commit and rm container when configuring", func() {
			runner := ddocker.DockerConfigureCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkConfigureCmd()
		})

		It("Should run all docker commands for full bootstrap", func() {
			runner := ddocker.DockerBootstrapCmd{Config: "test"}
			go runner.Run(cli, &ctx)
			checkBuildCmd()
			checkMigrateCmd()
			checkConfigureCmd()
		})
	})
})
