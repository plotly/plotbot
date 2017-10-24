package deployer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/plotly/plotbot"
	"github.com/plotly/plotbot/internal"
	"github.com/plotly/plotbot/testutils"
	"github.com/plotly/plotbot/util"
	"github.com/stretchr/testify/assert"
)

var TEST_CONFIRM_TIMEOUT = time.Second

func newTestDep(dconf DeployerConfig, bot plotbot.BotLike, runner Runnable) *Deployer {

	serviceConfigs := map[string]ServiceConfig{
		"streambed": {
			RepositoryPath:      "/usr/local",
			DefaultBranch:       "production",
			AllowedProdBranches: []string{"master"},
			InventoryArgs:       []string{"-i", "tools/plotly_gce"},
		},
		"testrepo": {
			RepositoryPath:      "/tmp",
			DefaultBranch:       "testbranch",
			AllowedProdBranches: []string{"master"},
		},
	}

	defaultdconf := DeployerConfig{
		AnnounceRoom: "#streambed",
		ProgressRoom: "#deploy",
		Services:     serviceConfigs,
	}

	if dconf.Services != nil {
		defaultdconf.Services = dconf.Services
	}
	if dconf.AnnounceRoom != "" {
		defaultdconf.AnnounceRoom = dconf.AnnounceRoom
	}
	if dconf.ProgressRoom != "" {
		defaultdconf.ProgressRoom = dconf.ProgressRoom
	}

	iconf := internal.InternalAPIConfig{
		"prod": {
			BaseURL: "https://example.com/internal/",
			AuthKey: "test_auth_key",
		},
	}
	iapi := internal.InternalAPI{
		Config: &iconf,
	}

	return &Deployer{
		config:         &defaultdconf,
		bot:            bot,
		runner:         runner,
		progress:       make(chan string, 1000),
		confirmTimeout: TEST_CONFIRM_TIMEOUT,
		internal:       &iapi,
	}
}

func defaultTestDep(cmdDelay time.Duration) *Deployer {
	return newTestDep(
		DeployerConfig{},
		testutils.NewDefaultMockBot(),
		&testutils.MockRunner{
			ParseVars: func(c string, s ...string) []string {
				switch c {
				case "ansible-playbook":
					return []string{
						"GO_CMD_PROCESS_OUTPUT={{ansible-output}}",
						fmt.Sprintf("GO_CMD_PROCESS_DELAY=%d", cmdDelay/time.Second),
					}
				default:
					return []string{}
				}
			},
		})
}

func captureProgress(dep *Deployer, waitTime time.Duration) (util.Searchable, error) {

	timer := time.NewTimer(waitTime)
	done := make(chan bool, 2)
	progress := util.Searchable{}
	for {
		select {
		case <-timer.C:
			return progress, fmt.Errorf("timer expired without progress")
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

	cwd, err := os.Getwd()
	if err == nil {
		fmt.Printf("GO_CMD_WD=%s\n", cwd)
	} else {
		fmt.Printf("Error determining working directory: %s\n", err)
	}

	output := os.Getenv("GO_CMD_PROCESS_OUTPUT")
	if output != "" {
		fmt.Println(output)
	}

	exitCode := os.Getenv("GO_CMD_PROCESS_EXIT")
	i, err = strconv.Atoi(exitCode)
	if err == nil {
		os.Exit(i)
	}
}

func TestCancelDeployNotRunning(t *testing.T) {
	dep := defaultTestDep(time.Second)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "cancel deploy"))

	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 1 {
		t.Fatalf("expected 1 reply found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := "No deploy running, sorry friend.."
	if actual != expected {
		t.Errorf("expected '%s' but found '%s'", expected, actual)
	}
}

func TestStageDeploy(t *testing.T) {
	dep := defaultTestDep(time.Second)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to stage"))

	progress, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := util.Searchable{
		"ansible-playbook -i tools/",
		"GO_CMD_WD=/usr/local",
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
		t.Errorf("expected '%s' to contain '%s'", expected, actual)
	}

	actual = bot.TestReplies[1].Text
	expected = fmt.Sprintf("<@%s> your deploy was successful", testutils.DefaultFromUser)
	if actual != expected {
		t.Errorf("expected '%s' but found '%s'", expected, actual)
	}
}

func TestProdDeployWithTags(t *testing.T) {
	dep := defaultTestDep(time.Second)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to prod, tags: umwelt"))

	progress, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := util.Searchable{"ansible-playbook -i tools/",
		"GO_CMD_WD=/usr/local",
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
		t.Errorf("expected '%s' to contain '%s'", expected, actual)
	}

	actual = bot.TestReplies[1].Text
	expected = fmt.Sprintf("<@%s> your deploy was successful", testutils.DefaultFromUser)
	if actual != expected {
		t.Errorf("expected '%s' but found '%s'", expected, actual)
	}
}

func TestDeployOtherService(t *testing.T) {
	dep := defaultTestDep(time.Second)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to testrepo prod"))

	progress, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := util.Searchable{"ansible-playbook playbook_prod.yml",
		"GO_CMD_WD=/tmp",
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
}

func TestDeployInvalidService(t *testing.T) {
	dep := defaultTestDep(0)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to invalid stage"))

	progress, err := captureProgress(dep, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := util.Searchable{
		"invalid is not a valid service",
	}

	if !progress.ContainsAll(expectContain...) {
		t.Errorf("expected progress %s to contain all of %s", progress.String(),
			expectContain.String())
	}

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies
	if len(replies) != 1 {
		t.Fatalf("expected 1 reply got %d", len(replies))
	}

	actual := replies[0].Text
	expected := "invalid is not a valid service"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected reply '%s' to contain '%s'", actual, expected)
	}
}

func TestLockUnlock(t *testing.T) {

	// First test locking - set command delay to 0 so we can wait for progress
	// on a shorter interval.
	dep := defaultTestDep(time.Second * 0)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "please lock deployment"))

	// there should be no progress
	_, err := captureProgress(dep, time.Millisecond*500)
	if err == nil {
		t.Errorf("expected timeout error while capturing non-existent progress")
	}

	runner := dep.runner.(*testutils.MockRunner)
	if len(runner.Jobs) != 0 {
		t.Fatalf("expected no job to be run found %d", len(runner.Jobs))
	}

	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 1 {
		t.Fatalf("expected 1 replies found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := "Deployment is now locked"
	if !strings.Contains(actual, expected) {
		t.Fatalf("expected '%s' to contain '%s'", expected, actual)
	}

	// Then make sure a deploy fails while locked
	clearMocks(dep)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsgFromUser(dep.bot, "deploy to prod", "rodoh"))

	_, err = captureProgress(dep, time.Millisecond*500)
	if err == nil {
		t.Errorf("expected timeout error while capturing non-existent progress")
	}

	if len(runner.Jobs) != 0 {
		t.Fatalf("expected no job to be run found %d", len(runner.Jobs))
	}

	if len(bot.TestReplies) != 1 {
		t.Fatalf("expected 1 replies found %d", len(bot.TestReplies))
	}

	actual = bot.TestReplies[0].Text
	expected = fmt.Sprintf("Deployment was locked by %s", testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Fatalf("expected '%s' to contain '%s'", expected, actual)
	}

	// Unlock deployment
	clearMocks(dep)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "unlock deployment"))

	_, err = captureProgress(dep, time.Millisecond*500)
	if err == nil {
		t.Errorf("expected timeout error while capturing non-existent progress")
	}

	if len(runner.Jobs) != 0 {
		t.Fatalf("expected no job to be run found %d", len(runner.Jobs))
	}

	if len(bot.TestReplies) != 1 {
		t.Fatalf("expected 1 replies found %d", len(bot.TestReplies))
	}

	actual = bot.TestReplies[0].Text
	expected = "Deployment is now unlocked"
	if !strings.Contains(actual, expected) {
		t.Fatalf("expected '%s' to contain '%s'", expected, actual)
	}

	// Finally make sure we can now deploy
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to prod"))
	captureProgress(dep, time.Millisecond*500)

	if len(runner.Jobs) != 3 {
		t.Fatalf("expected 3 job found %d", len(runner.Jobs))
	}
}

func TestCancelDeploy(t *testing.T) {

	// set up for long running deploy
	dep := defaultTestDep(time.Second * 5)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to stage"))

	time.Sleep(time.Millisecond * 500)

	fromUser := "rodoh"
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsgFromUser(dep.bot, "cancel deploy", fromUser))

	progress, err := captureProgress(dep, time.Second*4)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := util.Searchable{
		"ansible-playbook",
		"--tags updt_streambed",
		"terminated with error: signal: interrupt",
	}
	if !progress.ContainsAll(expectContain...) {
		t.Errorf("expected progress %s to contain all of %s", progress.String(),
			expectContain.String())
	}

	expectNotToContain := util.Searchable{
		"terminated successfully",
		"{{ansible-output}}",
	}
	if progress.ContainsAny(expectNotToContain...) {
		t.Errorf("expected progress %s not to contain any of %s", progress.String(),
			expectContain.String())
	}

	// 3 jobs should have run
	runner := dep.runner.(*testutils.MockRunner)
	if len(runner.Jobs) != 3 {
		t.Fatalf("expected 3 job found %d", len(runner.Jobs))
	}

	// should have made 3 replies
	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 3 {
		t.Fatalf("expected 3 replies found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[1].Text
	expected := "deploy: Sending Interrupt signal"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' to contain '%s'", actual, expected)
	}

	actual = bot.TestReplies[2].Text
	expected = fmt.Sprintf("<@%s> your deploy failed: signal: interrupt",
		testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' to contain '%s'", actual, expected)
	}
}

func TestJobAlreadyRunning(t *testing.T) {
	dep := defaultTestDep(time.Second)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to stage"))

	time.Sleep(time.Millisecond * 200)

	fromUser := "rodoh"
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsgFromUser(dep.bot, "deploy to prod", fromUser))

	_, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies
	if len(replies) != 3 {
		t.Fatalf("expected 3 replies got %d", len(replies))
	}

	actual := replies[1].Text
	expected := "Deploy currently running"
	if !(strings.Contains(actual, fromUser) && strings.Contains(actual, expected)) {
		t.Errorf("expected reply '%s' to contain '%s' and '%s'", actual, fromUser, expected)
	}

	actual = replies[2].Text
	expected = "deploy was successful"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected reply '%s' to contain '%s'", actual, expected)
	}
}

func TestHelp(t *testing.T) {
	dep := defaultTestDep(time.Second)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy whats up?"))

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies

	if len(replies) != 1 {
		t.Fatalf("expected 1 replies got %d", len(replies))
	}

	actual := replies[0].Text
	if !strings.Contains(strings.ToLower(actual), "usage") {
		t.Errorf("expected reply '%s' to contain '%s'", actual, "usage")
	}
	if !strings.Contains(strings.ToLower(actual), "examples") {
		t.Errorf("expected reply '%s' to contain '%s'", actual, "examples")
	}
}

func TestAllowedProdBranches(t *testing.T) {
	dep := defaultTestDep(time.Second * 0)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy cats to prod"))

	_, err := captureProgress(dep, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies

	if len(replies) != 1 {
		t.Fatalf("expected 1 replies got %d", len(replies))
	}

	actual := replies[0].Text
	expected := "cats is not a legal branch for prod"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected reply '%s' to contain '%s'", actual, expected)
	}

	clearMocks(dep)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy master to prod"))

	_, err = captureProgress(dep, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}

	bot = dep.bot.(*testutils.MockBot)
	replies = bot.TestReplies

	if len(replies) != 2 {
		t.Fatalf("expected 2 replies got %d", len(replies))
	}

	actual = replies[1].Text
	expected = "your deploy was successful"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected reply '%s' to contain '%s'", actual, expected)
	}
}

func TestFailedGitFetch(t *testing.T) {
	dep := newTestDep(
		DeployerConfig{},
		testutils.NewDefaultMockBot(),
		&testutils.MockRunner{
			ParseVars: func(c string, s ...string) []string {
				args := util.Searchable(s)
				if c == "git" && args.Contains("fetch") {
					return []string{"GO_CMD_PROCESS_EXIT=99"}
				}
				return []string{}
			},
		})

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to prod"))

	_, err := captureProgress(dep, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies

	if len(replies) != 1 {
		t.Fatalf("expected 1 reply got %d", len(replies))
	}

	actual := replies[0].Text
	expected := "Unable to pull from repo: Error executing git fetch: exit status 99"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected reply '%s' to contain '%s'", actual, expected)
	}
}

func TestFailedGitCheckout(t *testing.T) {
	dep := newTestDep(
		DeployerConfig{},
		testutils.NewDefaultMockBot(),
		&testutils.MockRunner{
			ParseVars: func(c string, s ...string) []string {
				args := util.Searchable(s)
				if c == "git" && args.Contains("checkout") {
					return []string{"GO_CMD_PROCESS_EXIT=99"}
				}
				return []string{}
			},
		})

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to prod"))

	_, err := captureProgress(dep, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies

	if len(replies) != 1 {
		t.Fatalf("expected 1 reply got %d", len(replies))
	}

	actual := replies[0].Text
	expected := "Unable to pull from repo: exit status 99"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected reply '%s' to contain '%s'", actual, expected)
	}
}

func TestFailedAnsible(t *testing.T) {
	dep := newTestDep(
		DeployerConfig{},
		testutils.NewDefaultMockBot(),
		&testutils.MockRunner{
			ParseVars: func(c string, s ...string) []string {
				if c == "ansible-playbook" {
					return []string{"GO_CMD_PROCESS_EXIT=99"}
				}
				return []string{}
			},
		})

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "deploy to prod, tags: onions"))

	progress, err := captureProgress(dep, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}

	if !progress.Contains("terminated") {
		t.Errorf("expected progress %s to contain 'terminated'", progress)
	}

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies

	if len(replies) != 2 {
		t.Fatalf("expected 2 replies got %d", len(replies))
	}

	actual := replies[1].Text
	expected := "your deploy failed: exit status 99"
	if !strings.Contains(actual, expected) {
		t.Errorf("expected reply '%s' to contain '%s'", actual, expected)
	}
}

func TestRunPlaybookConfirmationBlockingAndTimeout(t *testing.T) {
	dep := defaultTestDep(time.Second * 0)
	playbook := CONFIRM_PLAYBOOKS[0]

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot,
			fmt.Sprintf("run %s on stage", playbook)))

	time.Sleep(50 * time.Millisecond)

	otherUser := "rodoh"
	// attempt to confirm but a different user (should fail)
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsgFromUser(dep.bot, "yes", otherUser))

	time.Sleep(50 * time.Millisecond)

	// attempt to deploy. Should fail as we are waiting for confirmation
	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsgFromUser(dep.bot, "deploy to prod", otherUser))

	// check progress to make sure we don't receive any
	progress, err := captureProgress(dep, TEST_CONFIRM_TIMEOUT+50*time.Millisecond)
	if err == nil {
		fmt.Println(strings.Join(progress, "; "))
		t.Fatal("expected timeout error as we are expecting no progress")
	}

	runner := dep.runner.(*testutils.MockRunner)
	if len(runner.Jobs) != 0 {
		t.Fatalf("expected 0 job found %d", len(runner.Jobs))
	}

	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 3 {
		t.Fatalf("expected 3 replies found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := fmt.Sprintf("<@%s> This job requires confirmation. "+
		"Confirm with '@%s: [yes|no]'",
		testutils.DefaultFromUser, bot.Config.Nickname)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' to contain '%s'", expected, actual)
	}

	actual = bot.TestReplies[1].Text
	expected = fmt.Sprintf("<@%s> waiting for confirmation from %s",
		otherUser, testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' to contain '%s'", expected, actual)
	}

	actual = bot.TestReplies[2].Text
	expected = fmt.Sprintf("<@%s> Did not receive confirmation in time. "+
		"Cancelling job", testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' but found '%s'", expected, actual)
	}
}

func TestRunPlaybookConfirmationSuccess(t *testing.T) {
	dep := defaultTestDep(time.Second * 1)
	playbook := CONFIRM_PLAYBOOKS[0]

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot,
			fmt.Sprintf("run %s on stage", playbook)))

	time.Sleep(50 * time.Millisecond)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "yes I confirm"))

	progress, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := util.Searchable{
		"GO_CMD_WD=/usr/local",
		"playbook_stage_postgres_recovery.yml",
		"{{ansible-output}}",
		"terminated successfully",
	}

	if !progress.ContainsAll(expectContain...) {
		t.Errorf("expected progress %s to contain all of %s", progress.String(),
			expectContain.String())
	}

	expectNotToContain := util.Searchable{
		"tags",
	}

	if progress.ContainsAny(expectNotToContain...) {
		t.Errorf("expected progress %s not to contain any of %s", progress.String(),
			expectContain.String())
	}

	bot := dep.bot.(*testutils.MockBot)
	if len(bot.TestReplies) != 3 {
		t.Fatalf("expected 3 replies found %d", len(bot.TestReplies))
	}

	actual := bot.TestReplies[0].Text
	expected := fmt.Sprintf("<@%s> This job requires confirmation. "+
		"Confirm with '@%s: [yes|no]'",
		testutils.DefaultFromUser, bot.Config.Nickname)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' to contain '%s'", expected, actual)
	}

	actual = bot.TestReplies[1].Text
	expected = fmt.Sprintf("<@%s> deploying, my friend", testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' but found '%s'", actual, expected)
	}

	actual = bot.TestReplies[2].Text
	expected = fmt.Sprintf("<@%s> your deploy was successful",
		testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' but found '%s'", actual, expected)
	}
}

func TestRunHelp(t *testing.T) {
	dep := defaultTestDep(time.Second)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "run help"))

	bot := dep.bot.(*testutils.MockBot)
	replies := bot.TestReplies

	if len(replies) != 1 {
		t.Fatalf("expected 1 replies got %d", len(replies))
	}

	actual := replies[0].Text
	if !strings.Contains(strings.ToLower(actual), "usage") {
		t.Errorf("expected reply '%s' to contain '%s'", actual, "usage")
	}
	if !strings.Contains(strings.ToLower(actual), "examples") {
		t.Errorf("expected reply '%s' to contain '%s'", actual, "examples")
	}
	if !strings.Contains(strings.ToLower(actual), "postgres_failover") {
		t.Errorf("expected reply '%s' to contain '%s'", actual, "examples")
	}
}

func TestRunOtherService(t *testing.T) {
	dep := defaultTestDep(time.Second * 1)

	dep.ChatHandler(&plotbot.Conversation{Bot: dep.bot},
		testutils.ToBotMsg(dep.bot, "run testcmd on testrepo stage"))

	progress, err := captureProgress(dep, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	expectContain := util.Searchable{
		"GO_CMD_WD=/tmp",
		"ansible-playbook playbook_stage_testcmd.yml",
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
	expected := fmt.Sprintf("<@%s> deploying, my friend", testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' but found '%s'", actual, expected)
	}

	actual = bot.TestReplies[1].Text
	expected = fmt.Sprintf("<@%s> your deploy was successful",
		testutils.DefaultFromUser)
	if !strings.Contains(actual, expected) {
		t.Errorf("expected '%s' but found '%s'", actual, expected)
	}
}

func TestGetCompareUrl(t *testing.T) {
	dir, err := ioutil.TempDir("", "deptest")
	if err != nil {
		t.Fatalf("error creating temporary directory: %s", err)
	}

	defer os.RemoveAll(dir)

	dep := newTestDep(
		DeployerConfig{},
		testutils.NewDefaultMockBot(),
		&Runner{},
	)

	assert.Equal(t, "", dep.getCompareUrl("prod", "master", dir), "compare URL empty when in_the_pipe missing")

	os.Mkdir(path.Join(dir, "tools"), os.ModePerm)
	ioutil.WriteFile(path.Join(dir, "tools", "in_the_pipe"), []byte("#!/bin/bash\necho -n https://pipeurl\n"), 0755)

	assert.Equal(t, "", dep.getCompareUrl("unconfigured", "master", dir), "compare URL empty with unconfigured environment")

	assert.Equal(t, "https://pipeurl", dep.getCompareUrl("prod", "master", dir), "compare URL incorrect")
}
