package deployer

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/kr/pty"

	"github.com/plotly/plotbot"
	"github.com/plotly/plotbot/internal"
	"github.com/plotly/plotbot/util"
)

var helpText = `*Usage:* %s [please|insert reverence] deploy [<branch-name>] to <environment> [, tags: <ansible-playbook tags>, ..., ...]
*Examples:*
• %s please deploy to prod
• %s deploy thing-to-test to stage
• %s deploy complicated-thing to stage, tags: updt_streambed, blow_up_the_sun
*Other commands:*
• %s what's in the pipe? - show what's waiting to be deployed to prod
• %s lock deployment - prevent deployment until it's unlocked
• %s cancel deploy - cancel the currently running deployment`

type Deployer struct {
	runner         Runnable
	runningJob     *DeployJob
	bot            plotbot.BotLike
	confirmJob     *ConfirmJob
	confirmTimeout time.Duration
	env            string
	config         *DeployerConfig
	progress       chan string
	internal       *internal.InternalAPI
	lockedBy       string
}

type DeployerConfig struct {
	RepositoryPath      string   `json:"repository_path"`
	AnnounceRoom        string   `json:"announce_room"`
	ProgressRoom        string   `json:"progress_room"`
	DefaultBranch       string   `json:"default_branch"`
	AllowedProdBranches []string `json:"allowed_prod_branches"`
}

type Runnable interface {
	Run(string, ...string) *exec.Cmd
}

type Runner struct{}

func (r *Runner) Run(cmd string, args ...string) *exec.Cmd {
	return exec.Command(cmd, args...)
}

func init() {
	plotbot.RegisterPlugin(&Deployer{})
}

func (dep *Deployer) InitPlugin(bot *plotbot.Bot) {
	var conf struct {
		Deployer DeployerConfig
	}
	bot.LoadConfig(&conf)

	dep.bot = bot
	dep.progress = make(chan string, 1000)
	dep.config = &conf.Deployer
	dep.env = os.Getenv("PLOTLY_ENV")
	dep.runner = &Runner{}
	dep.confirmTimeout = DEFAULT_CONFIRM_TIMEOUT

	if dep.env == "" {
		dep.env = "debug"
	}

	dep.loadInternalAPI()

	go dep.forwardProgress()

	bot.ListenFor(&plotbot.Conversation{
		HandlerFunc:    dep.ChatHandler,
		MentionsMeOnly: true,
	})
}

func (dep *Deployer) loadInternalAPI() {
	dep.internal = internal.New(dep.bot.LoadConfig)
}

/**
 * Examples:
 *   deploy to stage, branch boo, tags boom, reload-streambed
 *   deploy to stage the branch santa-claus with tags boom, reload-streambed
 *   deploy on prod, branch boo with tags: ahuh, mama, papa
 *   deploy to stage the branch master
 *   deploy prod branch boo  // shortest form
 * or second regexp:
 *   deploy branch boo to stage
 *   deploy santa-claus to stage with tags: kaboom
 */

type DeployJob struct {
	process *os.Process
	params  *DeployParams
	quit    chan bool
	kill    chan bool
	killing bool
}

var DEFAULT_CONFIRM_TIMEOUT = 4 * time.Second

type ConfirmJob struct {
	params *DeployParams
	done   chan bool
}

var CONFIRM_PLAYBOOKS = util.Searchable{
	"postgres_recovery", "postgres_failover"}

var deployFormat = regexp.MustCompile(`deploy( ([a-zA-Z0-9_\.-]+))? to ([a-z_-]+)((,| with)? tags?:? ?(.+))?`)

var runFormat = regexp.MustCompile(`run\s+([a-zA-Z0-9_\.-]+)\s+on\s+([a-z_-]+)((,|\s*with)?\s+tags?:? ?(.+))?`)

func (dep *Deployer) ExtractDeployParams(msg *plotbot.Message) *DeployParams {

	if match := deployFormat.FindStringSubmatch(msg.Text); match != nil {
		tags := strings.Replace(match[6], " ", "", -1)
		if tags == "" {
			tags = "updt_streambed"
		}
		return &DeployParams{
			Environment:     match[3],
			Branch:          match[2],
			Tags:            tags,
			InitiatedBy:     msg.FromUser.RealName,
			From:            "chat",
			initiatedByChat: msg,
		}

	} else if match := runFormat.FindStringSubmatch(msg.Text); match != nil {
		return &DeployParams{
			Playbook:        match[1],
			Environment:     match[2],
			Tags:            match[5],
			InitiatedBy:     msg.FromUser.RealName,
			From:            "chat",
			initiatedByChat: msg,
			Confirm:         CONFIRM_PLAYBOOKS.Includes(match[1]),
		}
	}

	return nil
}

func (dep *Deployer) ChatHandler(conv *plotbot.Conversation, msg *plotbot.Message) {
	bot := conv.Bot
	// msgp := *msg
	// msg = &msgp

	if params := dep.ExtractDeployParams(msg); params != nil {
		if dep.lockedBy != "" {
			conv.Reply(msg, fmt.Sprintf("Deployment was locked by %s.  "+
				"Unlock with '%s, unlock deployment' if they're OK with it.",
				dep.lockedBy, dep.bot.AtMention()))

		} else if dep.runningJob != nil {
			dep.replyPersonnally(params,
				fmt.Sprintf("Deploy currently running: %s", dep.runningJob.params))

		} else if dep.confirmJob != nil {
			m := fmt.Sprintf(
				"waiting for confirmation from %s", dep.confirmJob.params.InitiatedBy,
			)
			dep.replyPersonnally(params, m)

		} else if params.Confirm {
			dep.confirmJob = &ConfirmJob{
				params: params,
				done:   make(chan bool, 2),
			}
			m := fmt.Sprintf("This job requires confirmation. "+
				"Confirm with '%s [yes|no]'", dep.bot.AtMention())
			dep.replyPersonnally(params, m)
			go dep.manageConfirm()

		} else {
			go dep.handleDeploy(params)
		}

	} else if msg.Contains("cancel deploy") {

		if dep.runningJob == nil {
			conv.Reply(msg, "No deploy running, sorry friend..")
		} else {
			if dep.runningJob.killing == true {
				conv.Reply(msg,
					"deploy: Interrupt signal already sent, waiting to die")
			} else {
				conv.Reply(msg, "deploy: Sending Interrupt signal...")
				dep.runningJob.killing = true
				dep.runningJob.kill <- true
			}
		}
	} else if msg.Contains("in the pipe") {
		url := dep.getCompareUrl("prod", dep.config.DefaultBranch)
		mention := msg.FromUser.Name
		if url != "" {
			conv.Reply(msg,
				fmt.Sprintf("@%s in %s branch, waiting to reach prod: %s",
					mention, dep.config.DefaultBranch, url))
		} else {
			conv.Reply(msg,
				fmt.Sprintf("@%s couldn't get current revision on prod", mention))
		}
	} else if msg.Contains("unlock deploy") {
		dep.lockedBy = ""
		conv.Reply(msg, fmt.Sprintf("Deployment is now unlocked."))
		bot.Notify(dep.config.AnnounceRoom, "#00ff00",
			fmt.Sprintf("%s has unlocked deployment", msg.FromUser.Name))

	} else if msg.Contains("lock deploy") {
		dep.lockedBy = msg.FromUser.Name
		conv.Reply(msg, fmt.Sprintf("Deployment is now locked.  "+
			"Unlock with '%s, unlock deployment' ASAP!", dep.bot.AtMention()))
		bot.Notify(dep.config.AnnounceRoom, "#ff0000",
			fmt.Sprintf("%s has locked deployment", dep.lockedBy))

	} else if msg.Contains("deploy") || msg.Contains("push to") {
		mention := dep.bot.AtMention()
		conv.Reply(msg, fmt.Sprintf(
			helpText, mention, mention, mention, mention, mention, mention, mention,
		))

	} else if dep.confirmJob != nil {
		waitingFor := dep.confirmJob.params.InitiatedBy
		msgFrom := msg.FromUser.RealName

		if waitingFor == msgFrom && msg.Contains("no") {
			dep.replyPersonnally(dep.confirmJob.params, "ok cancelling...")
			dep.confirmJob.done <- true

		} else if waitingFor == msgFrom && msg.Contains("yes") {
			go dep.handleDeploy(dep.confirmJob.params)
			dep.confirmJob.done <- true
		}
	}
}

func (dep *Deployer) handleDeploy(params *DeployParams) {
	hostsFile := fmt.Sprintf("hosts_%s", params.Environment)
	if params.Environment == "prod" {
		hostsFile = "tools/plotly_ec2.py"
	} else if params.Environment == "stage" {
		hostsFile = "tools/plotly_gce"
	}

	playbookFile := fmt.Sprintf("playbook_%s.yml", params.Environment)
	if params.Playbook != "" {
		playbookFile = fmt.Sprintf(
			"playbook_%s_%s.yml", params.Environment, params.Playbook,
		)
	} else if params.Environment == "stage" {
		playbookFile = "playbook_gcpstage.yml"
	}

	cmdArgs := []string{"ansible-playbook", "-i", hostsFile, playbookFile}
	if params.Tags != "" {
		cmdArgs = append(cmdArgs, "--tags", params.Tags)
	}

	branch := dep.config.DefaultBranch
	if params.Branch != "" {
		if params.Environment == "prod" {
			ok := false
			for _, allowed := range dep.config.AllowedProdBranches {
				if allowed == params.Branch {
					ok = true
					break
				}
			}
			if !ok {
				errorMsg := fmt.Sprintf(
					"%s is not a legal branch for prod.  Aborting.", params.Branch)
				dep.pubLine(fmt.Sprintf("[deployer] %s", errorMsg))
				dep.replyPersonnally(params, errorMsg)
				return
			}
		}
		branch = params.Branch
		pr := fmt.Sprintf("streambed_pull_revision=origin/%s", params.Branch)
		cmdArgs = append(cmdArgs, "-e", pr)
	}

	if err := dep.pullRepo(branch); err != nil {
		errorMsg := fmt.Sprintf("Unable to pull from repo: %s. Aborting.", err)
		dep.pubLine(fmt.Sprintf("[deployer] %s", errorMsg))
		dep.replyPersonnally(params, errorMsg)
		return
	} else {
		lr := fmt.Sprintf("[deployer] Using latest revision of %s branch", branch)
		dep.pubLine(lr)
	}

	bot := dep.bot
	bot.Notify(dep.config.AnnounceRoom, "#447bdc",
		fmt.Sprintf("[deployer] Launching: %s, monitor in %s",
			params, dep.config.ProgressRoom))
	dep.replyPersonnally(params, bot.WithMood(
		"deploying, my friend", "deploying, yyaaahhhOooOOO!"))

	if params.Environment == "prod" {
		url := dep.getCompareUrl(params.Environment, params.Branch)
		if url != "" {
			dep.pubLine(
				fmt.Sprintf("[deployer] Compare what is being pushed: %s", url))
		}
	}

	dep.pubLine(
		fmt.Sprintf("[deployer] Running cmd: %s", strings.Join(cmdArgs, " ")))

	cmd := dep.runner.Run(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = dep.config.RepositoryPath
	env := append(os.Environ(), "ANSIBLE_NOCOLOR=1")
	if cmd.Env != nil {
		env = append(env, cmd.Env...)
	}
	cmd.Env = env

	pty, err := pty.Start(cmd)
	if err != nil {
		log.Fatal(err)
	}

	dep.runningJob = &DeployJob{
		process: cmd.Process,
		params:  params,
		quit:    make(chan bool, 2),
		kill:    make(chan bool, 2),
	}

	go dep.manageDeployIo(pty)
	go dep.manageKillProcess(pty)

	if err := cmd.Wait(); err != nil {
		dep.pubLine(fmt.Sprintf("[deployer] terminated with error: %s", err))
		dep.replyPersonnally(params, fmt.Sprintf("your deploy failed: %s", err))

	} else {
		dep.pubLine("[deployer] terminated successfully")
		dep.replyPersonnally(params,
			bot.WithMood("your deploy was successful",
				"your deploy was GREAT, you're great !"))
	}

	dep.runningJob.quit <- true
	dep.runningJob = nil
}

func (dep *Deployer) pullRepo(branch string) error {
	cmd := dep.runner.Run("git", "fetch")
	cmd.Dir = dep.config.RepositoryPath
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Error executing git fetch: %s", err)
	}
	cmd = dep.runner.Run("git", "checkout", fmt.Sprintf("origin/%s", branch))
	cmd.Dir = dep.config.RepositoryPath
	return cmd.Run()
}

func (dep *Deployer) pubLine(str string) {
	dep.progress <- str
}

func (dep *Deployer) manageKillProcess(pty *os.File) {
	runningJob := dep.runningJob
	select {
	case <-runningJob.quit:
		return
	case <-runningJob.kill:
		dep.runningJob.process.Signal(os.Interrupt)
		time.Sleep(3 * time.Second)
		if dep.runningJob != nil {
			dep.runningJob.process.Kill()
		}
	}
}

func (dep *Deployer) manageConfirm() {
	confirmJob := dep.confirmJob
	select {
	case <-confirmJob.done:
		dep.confirmJob = nil
	case <-time.After(dep.confirmTimeout):
		m := fmt.Sprintf("Did not receive confirmation in time. "+
			"Cancelling job %s", confirmJob.params)
		dep.replyPersonnally(confirmJob.params, m)
		dep.confirmJob = nil
	}
}

func (dep *Deployer) forwardProgress() {
	lines := ""

	for {
		select {
		case msg := <-dep.progress:
			if msg != "" {
				lines += fmt.Sprintf("%s", msg)
			}
			lines += "\n"
		case <-time.After(2 * time.Second):
			if lines != "" {
				escapedLines := fmt.Sprintf("```%s```", lines)
				dep.bot.SendToChannel(dep.config.ProgressRoom, escapedLines)
				lines = ""
			}
		}
	}
}

func (dep *Deployer) manageDeployIo(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if dep.runningJob == nil {
			continue
		}
		dep.progress <- scanner.Text()
	}
}

func (dep *Deployer) replyPersonnally(params *DeployParams, msg string) {
	if params.initiatedByChat == nil {
		return
	}
	dep.bot.ReplyMention(params.initiatedByChat, msg)
}

func (dep *Deployer) getCompareUrl(env, branch string) string {
	if dep.internal == nil {
		return ""
	}

	currentHead := dep.internal.GetCurrentHead(env)
	if currentHead == "" {
		return ""
	}

	url := fmt.Sprintf("https://github.com/plotly/streambed/compare/%s...%s",
		currentHead, branch)
	return url
}
