package deployer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/plotly/plotbot"
	"github.com/plotly/plotbot/testutils"
)

func newTestDep(dconf DeployerConfig, bot plotbot.BotLike) *Deployer {

	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	defaultdconf := DeployerConfig{
		RepositoryPath:      filepath.Dir(execPath),
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
		runner: &testutils.MockRunner{
			ParseVars: func(c string, s ...string) []string {
				switch c {
				case "ansible-playbook":
					return []string{
						"GO_CMD_PROCESS_OUTPUT={{ansible-output}}",
						"GO_CMD_PROCESS_DELAY=1",
					}
				default:
					return []string{}
				}
			},
		},
		progress: make(chan string, 1000),
	}
}

func defaultTestDep() *Deployer {
	return newTestDep(DeployerConfig{}, testutils.NewDefaultMockBot())
}

func captureProgress(dep *Deployer, waitTime time.Duration) (testutils.Searchable, error) {

	timer := time.NewTimer(waitTime)
	done := make(chan bool, 2)
	progress := testutils.Searchable{}
	for {
		select {
		case <-timer.C:
			return nil, fmt.Errorf("timer expired without progress")
		case <-done:
			return progress, nil
		case p := <-dep.progress:
			progress = append(progress, p)

			// if we get some progress we can assume runningJob is active
			// and if it subsequently becomes nil we can assume the job is
			// complete and we can finish waiting for progress.
			if len(progress) == 1 {
				go func() {
					ticker := time.NewTicker(time.Millisecond * 100)
					for _ = range ticker.C {
						if dep.runningJob == nil {
							ticker.Stop()
							done <- true
						}
					}
				}()
			}
		}
	}
}

func clearMocks(dep *Deployer) {
	testutils.ClearMockBot(dep.bot.(*testutils.MockBot))
	testutils.ClearMockRunner(dep.runner.(*testutils.MockRunner))
}

// This test is called by the the mock cmd.Run() or pty.Start(cmd)
func TestCmdProcess(t *testing.T) {

	if os.Getenv("GO_WANT_CMD_PROCESS") != "1" {
		return
	}

	delay := os.Getenv("GO_CMD_PROCESS_DELAY")
	i, err := strconv.Atoi(delay)
	if err == nil {
		time.Sleep(time.Second * time.Duration(i))
	}

	output := os.Getenv("GO_CMD_PROCESS_OUTPUT")
	if output != "" {
		fmt.Println(output)
	}
}

func TestCancelDeployNotRunning(t *testing.T) {
	dep := defaultTestDep()

	conv := plotbot.Conversation{
		Bot: dep.bot,
	}
	msg := testutils.ToBotMsg(dep.bot, "cancel deploy")

	dep.ChatHandler(&conv, &msg)

	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 1 {
		t.Fatalf("expected 1 reply found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := "No deploy running, sorry friend.."
	if actual != expected {
		t.Errorf("exected '%s' but found '%s'", expected, actual)
	}
}

func TestStageDeploy(t *testing.T) {
	dep := defaultTestDep()

	conv := plotbot.Conversation{
		Bot: dep.bot,
	}
	msg := testutils.ToBotMsg(dep.bot, "deploy to stage")

	dep.ChatHandler(&conv, &msg)
	progress, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := testutils.Searchable{"ansible-playbook -i tools/",
		"--tags updt_streambed",
		"{{ansible-output}}",
		"terminated successfully",
	}

	if !progress.ContainsAll(expectContain...) {
		t.Errorf("expected progress %s to contain all of %s", progress.String(),
			expectContain.String())
	}

	runner := dep.runner.(*testutils.MockRunner)
	if len(runner.Jobs) != 3 {
		t.Fatalf("expected 3 job found %d", len(runner.Jobs))
	}

	if !(runner.Jobs[0].Contains("git") && runner.Jobs[1].Contains("git")) {
		t.Fatalf("expected first two jobs to be git jobs (fetch then pull)")
	}

	if !runner.Jobs[2].Contains("ansible-playbook") {
		t.Fatalf("expected last job to be ansible job")
	}

	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 2 {
		t.Fatalf("expected 2 replies found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := fmt.Sprintf("<@%s> deploying", testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("exected '%s' to contain '%s'", expected, actual)
	}

	actual = bot.TestReplies[1].Text
	expected = fmt.Sprintf("<@%s> your deploy was successful", testutils.DefaultFromUser)
	if actual != expected {
		t.Errorf("exected '%s' but found '%s'", expected, actual)
	}
}

func TestProdDeployWithTags(t *testing.T) {
	dep := defaultTestDep()

	conv := plotbot.Conversation{
		Bot: dep.bot,
	}
	msg := testutils.ToBotMsg(dep.bot, "deploy to prod with tags: umwelt")

	dep.ChatHandler(&conv, &msg)
	progress, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := testutils.Searchable{"ansible-playbook -i tools/",
		"--tags umwelt",
		"{{ansible-output}}",
		"terminated successfully",
	}

	if !progress.ContainsAll(expectContain...) {
		t.Errorf("expected progress %s to contain all of %s", progress.String(),
			expectContain.String())
	}

	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 2 {
		t.Fatalf("expected 2 replies found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := fmt.Sprintf("<@%s> deploying", testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("exected '%s' to contain '%s'", expected, actual)
	}

	actual = bot.TestReplies[1].Text
	expected = fmt.Sprintf("<@%s> your deploy was successful", testutils.DefaultFromUser)
	if actual != expected {
		t.Errorf("exected '%s' but found '%s'", expected, actual)
	}
}

