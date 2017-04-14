package deployer

import (
	"os"
	"strconv"
	"testing"

	"github.com/plotly/plotbot"
	"github.com/plotly/plotbot/testutils"
)

func newTestDep(dconf DeployerConfig, bot plotbot.BotLike) *Deployer {

	defaultdconf := DeployerConfig{
		RepositoryPath:      "/path/to/streambed/deployment",
		AnnounceRoom:        "#streambed",
		ProgressRoom:        "#deploy",
		DefaultBranch:       "production",
		AllowedProdBranches: []string{"master"},
	}

	if dconf.RepositoryPath != "" {
		defaultdconf.RepositoryPath = dconf.RepositoryPath
	}
	if dconf.AnnounceRoom != "" {
		defaultdconf.AnnounceRoom = dconf.AnnounceRoom
	}
	if dconf.ProgressRoom != "" {
		defaultdconf.ProgressRoom = dconf.ProgressRoom
	}
	if dconf.DefaultBranch != "" {
		defaultdconf.DefaultBranch = dconf.DefaultBranch
	}
	if len(dconf.AllowedProdBranches) != 0 {
		defaultdconf.AllowedProdBranches = dconf.AllowedProdBranches
	}

	return &Deployer{
		config: &defaultdconf,
		bot:    bot,
		runner: testutils.MockRunner{},
	}
}

func defaultTestDep() *Deployer {
	return newTestDep(DeployerConfig{}, testutils.NewDefaultMockBot())
}

func TestCmdProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CMD_PROCESS") != "1" {
		return
	}

	exitCode := os.Getenv("GO_CMD_PROCESS_EXIT_CODE")
	i, err := strconv.Atoi(exitCode)
	if err != nil {
		t.Error(err)
	} else {
		os.Exit(i)
	}
}

func TestCancelDeployNotRunning(t *testing.T) {
	dep := defaultTestDep()

	conv := plotbot.Conversation{
		Bot: dep.bot,
	}
	msg := testutils.ToBotMsg(dep.bot, "cancel deploy")

	dep.ChatHandler(&conv, &msg)

	bot := dep.bot.(*testutils.Bot)
	if len(bot.TestReplies) != 1 {
		t.Errorf("expected 1 reply found %s", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := "No deploy running, sorry friend.."
	if actual != expected {
		t.Errorf("exected '%s' but found '%s'", expected, actual)
	}
}
