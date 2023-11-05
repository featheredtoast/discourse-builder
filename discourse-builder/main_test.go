package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bytes"
	"context"
	ddocker "github.com/discourse/discourse_docker/discourse-builder"
	"io"
	"os"
	"os/exec"
	"strings"
)

type FakeCmdRunner struct {
	Cmd      *exec.Cmd
	RunCalls chan int
}

func (r *FakeCmdRunner) Run() error {
	r.RunCalls <- 1
	return nil
}

// Swap out CmdRunner with a fake instance that also returns created ICmdRunners on a channel
// so tests can inspect commands the moment they're run
func CreateNewFakeCmdRunner(c chan ddocker.ICmdRunner) func(cmd *exec.Cmd) ddocker.ICmdRunner {
	return func(cmd *exec.Cmd) ddocker.ICmdRunner {
		cmdRunner := &FakeCmdRunner{Cmd: cmd, RunCalls: make(chan int)}
		c <- cmdRunner
		return cmdRunner
	}
}

var _ = Describe("Main", func() {
	var testDir string
	var out *bytes.Buffer
	var cli *ddocker.Cli
	var ctx context.Context

	BeforeEach(func() {
		out = &bytes.Buffer{}
		ddocker.Out = out
		testDir, _ = os.MkdirTemp("", "ddocker-test")

		ctx = context.Background()

		cli = &ddocker.Cli{
			ConfDir:      "./test/containers",
			TemplatesDir: "./test",
			OutputDir:    testDir,
			ContainerId: "discourse-build-asdf",
		}
	})
	AfterEach(func() {
		os.RemoveAll(testDir)
	})

	It("should allow concatenated templates", func() {
		runner := ddocker.RawYamlCmd{Config: "test"}
		runner.Run(cli)
		Expect(out.String()).To(ContainSubstring("DISCOURSE_DEVELOPER_EMAILS: 'me@example.com,you@example.com'"))
		Expect(out.String()).To(ContainSubstring("_FILE_SEPERATOR_"))
		Expect(out.String()).To(ContainSubstring("version: tests-passed"))
	})

	It("should output docker compose cmd to config name's subdir", func() {
		runner := ddocker.DockerComposeCmd{Config: "test"}
		err := runner.Run(cli, &ctx)
		Expect(err).To(BeNil())
		out, err := os.ReadFile(testDir + "/test/config.yaml")
		Expect(err).To(BeNil())
		Expect(string(out[:])).To(ContainSubstring("DISCOURSE_DEVELOPER_EMAILS: 'me@example.com,you@example.com'"))
	})

	It("does not create output parent folders when not asked", func() {
		runner := ddocker.DockerComposeCmd{Config: "test"}
		cli.OutputDir = testDir + "/subfolder/sub-subfolder"
		err := runner.Run(cli, &ctx)
		Expect(err).ToNot(BeNil())
		_, err = os.ReadFile(testDir + "/subfolder/sub-subfolder/test/config.yaml")
		Expect(err).ToNot(BeNil())
	})

	It("should force create output parent folders when asked", func() {
		runner := ddocker.DockerComposeCmd{Config: "test"}
		cli.ForceMkdir = true
		cli.OutputDir = testDir + "/subfolder/sub-subfolder"
		err := runner.Run(cli, &ctx)
		Expect(err).To(BeNil())
		out, err := os.ReadFile(testDir + "/subfolder/sub-subfolder/test/config.yaml")
		Expect(err).To(BeNil())
		Expect(string(out[:])).To(ContainSubstring("DISCOURSE_DEVELOPER_EMAILS: 'me@example.com,you@example.com'"))
	})

	It("should clean after the command", func() {
		runner := ddocker.DockerComposeCmd{Config: "test"}
		runner.Run(cli, &ctx)
		runner2 := ddocker.CleanCmd{Config: "test"}
		err := runner2.Run(cli)
		Expect(err).To(BeNil())
		_, err = os.ReadFile(testDir + "/test/config.yaml")
		Expect(err).ToNot(BeNil())
	})

	Context("When running docker commands", func() {

		var CmdCreatorWatcher chan ddocker.ICmdRunner
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
			Expect(cmd.Cmd.Env).To(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))
			buf := new(strings.Builder)
			io.Copy(buf, cmd.Cmd.Stdin)
			// docker build's stdin is a dockerfile
			Expect(buf.String()).To(ContainSubstring("COPY config.yaml /temp-config.yaml"))
			Expect(buf.String()).To(ContainSubstring("--skip-tags=precompile,migrate,db"))
		}

		var checkMigrateCmd = func() {
			cmd := getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("-e DISCOURSE_DEVELOPER_EMAILS"))
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
			Expect(cmd.Cmd.String()).To(ContainSubstring("-e DISCOURSE_DEVELOPER_EMAILS"))
			// we commit, we need the container to stick around after it is stopped.
			Expect(cmd.Cmd.String()).ToNot(ContainSubstring("--rm"))
			Expect(cmd.Cmd.Env).To(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))
			buf := new(strings.Builder)
			io.Copy(buf, cmd.Cmd.Stdin)
			// docker run's stdin is a pups config
			Expect(buf.String()).To(ContainSubstring("path: /etc/service/nginx/run"))

			// commit on configure
			cmd = getLastCommand()
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker commit"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("--change CMD /sbin/boot"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("discourse-build"))
			Expect(cmd.Cmd.String()).To(ContainSubstring("local_discourse/test"))
			Expect(cmd.Cmd.Env).ToNot(ContainElement("DISCOURSE_DB_PASSWORD=SOME_SECRET"))
		}

		BeforeEach(func() {
			CmdCreatorWatcher = make(chan ddocker.ICmdRunner)
			ddocker.CmdRunner = CreateNewFakeCmdRunner(CmdCreatorWatcher)
		})
		AfterEach(func() {
			close(CmdCreatorWatcher)
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
