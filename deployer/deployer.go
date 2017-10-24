package deployer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kr/pty"

	"github.com/plotly/plotbot"
	"github.com/plotly/plotbot/internal"
	"github.com/plotly/plotbot/util"
)

func deployHelp(botName string) string {
	t := `*Usage:* %[1]s [please|insert reverence] deploy [<branch-or-image>] to <service> <environment> [, tags: <ansible-playbook-tags>, ..., ...]
<branch-or-image> is a git branch (technically a committish)
<service> defaults to streambed. imageserver is also supported.
<environment> is prod or stage
*Examples:*
• %[1]s please deploy to prod
• %[1]s deploy thing-to-test to stage
• %[1]s deploy to imageserver prod
• %[1]s deploy test-branch to imageserver stage
• %[1]s deploy complicated-thing to stage, tags: updt_streambed, blow_up_the_sun
*Other commands:*
• %[1]s what's in the pipe? - show what's waiting to be deployed to prod
• %[1]s lock deployment - prevent deployment until it's unlocked
• %[1]s cancel deploy - cancel the currently running deployment
• %[1]s run help - show help on running specific playbooks in an environment`
	return fmt.Sprintf(t, botName)
}
func runHelp(botName string, services map[string]ServiceConfig) string {
	t := `*Usage:* %[1]s [please|insert reverence] run [<playbook suffix>] in <service> <environment> [, tags: <ansible-playbook tags>, ..., ...]
*Examples:*
• %[1]s run postgres_failover on prod
• %[1]s run update_plotlyjs on imageserver prod
• %[1]s run postgres_recovery on streambed stage, tags: everything_is_broken`

	for service, serviceArgs := range services {
		playbooks, err := listAllowedPlaybooks(serviceArgs.RepositoryPath)
		if err == nil && len(playbooks) > 0 {
			t = t + fmt.Sprintf("\n\n*Available commands for %s:*", service)
			for _, env := range []string{"prod", "stage"} {
				envPlays := playbooks.ByEnvironment(env)
				if len(envPlays) > 0 {
					t = t + fmt.Sprintf("\n*%s*\n%s", env, envPlays.ToBullets())
				}
			}
		}
	}

	return fmt.Sprintf(t, botName)
}

var DEFAULT_CONFIRM_TIMEOUT = 30 * time.Second
var CONFIRM_PLAYBOOKS = util.Searchable{
	"postgres_recovery", "postgres_failover"}

var deployFormat = regexp.MustCompile(`deploy(?: ([a-zA-Z0-9_\.-]+))? to (?:([a-z_-]+) )?([a-z_-]+)(?:,\s+tags?:? ?(.+))?`)

var runFormat = regexp.MustCompile(`run\s+([a-zA-Z0-9_\.-]+)\s+on\s+(?:([a-z_-]+)\s+)?([a-z_-]+)(?:,\s+tags?:? ?(.+))?`)

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

type ServiceConfig struct {
	RepositoryPath      string   `json:"repository_path"`
	DefaultBranch       string   `json:"default_branch"`
	AllowedProdBranches []string `json:"allowed_prod_branches"`
	InventoryArgs       []string `json:"inventory_args"`
}

type DeployerConfig struct {
	AnnounceRoom string                   `json:"announce_room"`
	ProgressRoom string                   `json:"progress_room"`
	Services     map[string]ServiceConfig `json:"services"`
}

type DeployJob struct {
	process *os.Process
	params  *DeployParams
	quit    chan bool
	kill    chan bool
	killing bool
}

type ConfirmJob struct {
	params *DeployParams
	done   chan bool
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

func (dep *Deployer) ExtractDeployParams(msg *plotbot.Message) *DeployParams {

	if match := deployFormat.FindStringSubmatch(msg.Text); match != nil {
		service := match[2]
		if service == "" {
			service = "streambed"
		}
		tags := strings.Replace(match[4], " ", "", -1)
		if tags == "" && service == "streambed" {
			tags = "updt_streambed"
		}
		return &DeployParams{
			Service:         service,
			Environment:     match[3],
			Branch:          match[1],
			Tags:            tags,
			InitiatedBy:     msg.FromUser.RealName,
			From:            "chat",
			initiatedByChat: msg,
		}

	} else if match := runFormat.FindStringSubmatch(msg.Text); match != nil {
		service := match[2]
		if service == "" {
			service = "streambed"
		}
		return &DeployParams{
			Service:         service,
			Playbook:        match[1],
			Environment:     match[3],
			Tags:            match[4],
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
		url := dep.getCompareUrl("prod", dep.config.Services["streambed"].DefaultBranch, dep.config.Services["streambed"].RepositoryPath)
		mention := msg.FromUser.Name
		if url != "" {
			conv.Reply(msg,
				fmt.Sprintf("@%s in %s branch, waiting to reach prod: %s",
					mention, dep.config.Services["streambed"].DefaultBranch, url))
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
		conv.Reply(msg, deployHelp(dep.bot.AtMention()))

	} else if msg.Contains("run") && msg.ContainsAny([]string{"how", "help"}) {
		conv.Reply(msg, runHelp(dep.bot.AtMention(), dep.config.Services))

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
	// primary deployer syntax
	playbookFile := fmt.Sprintf("playbook_%s.yml", params.Environment)
	if params.Playbook != "" {

		// support "@plotbot run" syntax where specific playbooks may be named
		playbookFile = fmt.Sprintf(
			"playbook_%s_%s.yml", params.Environment, params.Playbook,
		)

	} else if params.Environment == "stage" || params.Environment == "prod" {

		playbookFile = fmt.Sprintf("playbook_%s.yml", params.Environment)
	}

	service := params.Service
	serviceArgs, found := dep.config.Services[service]

	if !found {
		errorMsg := fmt.Sprintf("%s is not a valid service.  Aborting.", params.Service)
		dep.pubLine(fmt.Sprintf("[deployer] %s", errorMsg))
		dep.replyPersonnally(params, errorMsg)
		return
	}

	cmdArgs := make([]string, 0)
	cmdArgs = append(cmdArgs, "ansible-playbook")
	cmdArgs = append(cmdArgs, serviceArgs.InventoryArgs...)
	cmdArgs = append(cmdArgs, playbookFile)

	if params.Tags != "" {
		cmdArgs = append(cmdArgs, "--tags", params.Tags)
	}

	branch := serviceArgs.DefaultBranch
	if params.Branch != "" {
		if params.Environment == "prod" {
			ok := false
			for _, allowed := range serviceArgs.AllowedProdBranches {
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
		pr := fmt.Sprintf("%s_pull_revision=origin/%s", service, params.Branch)
		cmdArgs = append(cmdArgs, "-e", pr)
	}

	if err := dep.pullRepo(branch, serviceArgs.RepositoryPath); err != nil {
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

	url := dep.getCompareUrl(params.Environment, params.Branch, serviceArgs.RepositoryPath)
	if url != "" {
		dep.pubLine(
			fmt.Sprintf("[deployer] Compare what is being pushed: %s", url))
	}

	dep.pubLine(
		fmt.Sprintf("[deployer] Running cmd: %s", strings.Join(cmdArgs, " ")))

	cmd := dep.runner.Run(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = serviceArgs.RepositoryPath
	env := append(os.Environ(), "ANSIBLE_NOCOLOR=1")
	if cmd.Env != nil {
		env = append(env, cmd.Env...)
	}
	cmd.Env = env

	err := dep.runWithOutput(cmd, params)

	if err != nil {
		dep.pubLine(fmt.Sprintf("[deployer] terminated with error: %s", err))
		dep.replyPersonnally(params, fmt.Sprintf("your deploy failed: %s", err))

		return
	}

	wd := filepath.Join(serviceArgs.RepositoryPath, "tools/watch_deployment")
	if _, err := os.Stat(wd); !os.IsNotExist(err) {
		cmd = dep.runner.Run(wd)
		cmd.Dir = serviceArgs.RepositoryPath

		err := dep.runWithOutput(cmd, params)

		if err != nil {
			dep.pubLine(fmt.Sprintf("[deployer] terminated with error: %s", err))
			dep.replyPersonnally(params, fmt.Sprintf("your deploy failed: %s", err))

			return
		}
	}

	dep.pubLine("[deployer] terminated successfully")
	dep.replyPersonnally(params,
		bot.WithMood("your deploy was successful",
			"your deploy was GREAT, you're great !"))
	return
}

func (dep *Deployer) runWithOutput(cmd *exec.Cmd, params *DeployParams) error {
	f, err := pty.Start(cmd)

	if err != nil {
		return err
	}

	dep.runningJob = &DeployJob{
		process: cmd.Process,
		params:  params,
		quit:    make(chan bool, 2),
		kill:    make(chan bool, 2),
	}

	go dep.manageDeployIo(f)
	go dep.manageKillProcess(f)

	err = cmd.Wait()

	dep.runningJob.quit <- true
	dep.runningJob = nil

	if err != nil {
		return err
	}

	return nil
}

func (dep *Deployer) pullRepo(branch, path string) error {
	cmd := dep.runner.Run("git", "fetch")
	cmd.Dir = path
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Error executing git fetch: %s", err)
	}
	cmd = dep.runner.Run("git", "checkout", fmt.Sprintf("origin/%s", branch))
	cmd.Dir = path
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

func (dep *Deployer) getCompareUrl(env, branch, path string) string {
	itp := filepath.Join(path, "tools/in_the_pipe")
	if _, err := os.Stat(itp); os.IsNotExist(err) {
		return ""
	}

	intConf, exists := (*dep.internal.Config)[env]
	if !exists {
		return ""
	}

	cmd := dep.runner.Run(itp, intConf.BaseURL, intConf.AuthKey, env, branch)
	cmd.Dir = path

	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	return string(out)
}
