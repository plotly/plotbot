package deployer

import (
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
	}
}

func defaultTestDep() *Deployer {
	return newTestDep(DeployerConfig{}, testutils.NewDefaultMockBot())
}

func TestCancelDeploy(t *testing.T) {
	dep := defaultTestDep()
	if dep.bot.Nickname() != "mockbot" {
		t.Errorf("expected Nickname to be %s, got %s instead",
			"mockbot", dep.bot.Nickname())
	}
}
