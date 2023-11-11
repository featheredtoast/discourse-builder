package docker_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bytes"
	"context"
	"github.com/discourse/discourse_docker/launcher_go/v2/config"
	"github.com/discourse/discourse_docker/launcher_go/v2/docker"
	. "github.com/discourse/discourse_docker/launcher_go/v2/test_utils"
	"github.com/discourse/discourse_docker/launcher_go/v2/utils"
	"strings"
)

var _ = Describe("Commands", func() {
	Context("under normal conditions", func() {
		var conf *config.Config
		var out *bytes.Buffer
		var ctx context.Context
		var CmdCreatorWatcher chan utils.ICmdRunner

		BeforeEach(func() {
			CmdCreatorWatcher = make(chan utils.ICmdRunner)
			utils.DockerPath = "docker"
			out = &bytes.Buffer{}
			utils.Out = out
			utils.CommitWait = 0
			conf = &config.Config{Name: "test"}
			ctx = context.Background()
			utils.CmdRunner = CreateNewFakeCmdRunner(CmdCreatorWatcher)
		})
		It("Removes unspecified image tags on commit", func() {
			runner := docker.DockerPupsRunner{Config: conf, ContainerId: "123", Ctx: &ctx, SavedImageName: "local_discourse/test:"}
			go runner.Run()
			cmd := GetLastCommand(CmdCreatorWatcher)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker run"))
			cmd = GetLastCommand(CmdCreatorWatcher)
			Expect(cmd.Cmd.String()).To(ContainSubstring("docker commit"))
			Expect(strings.HasSuffix(cmd.Cmd.String(), ":")).To(BeFalse())
		})
	})
})
