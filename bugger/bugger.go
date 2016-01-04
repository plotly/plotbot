package bugger

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/plotly/plotbot"
	"github.com/plotly/plotbot/github"
	"github.com/plotly/plotbot/util"
)

const dfltReportLength = 7 // days

func init() {
	plotbot.RegisterPlugin(&Bugger{})
}

type Bugger struct {
	bot      *plotbot.Bot
	ghclient github.Client
}

func (bugger *Bugger) makeBugReporter(days int, repo string) (reporter bugReporter) {




	query := github.SearchQuery{
		Repo:        repo,
		Labels:      []string{"bug"},
		ClosedSince: time.Now().Add(-time.Duration(days) * (24 * time.Hour)).Format("2006-01-02"),
	}

	issueList, err := bugger.ghclient.DoSearchQuery(query)
	if err != nil {
		log.Print(err)
		return
	}

	/*
	 * Get an array of issues matching Filters
	 */
	issueChan := make(chan github.IssueItem, 1)
	go bugger.ghclient.DoEventQuery(issueList, repo, issueChan)

	reporter := new(bugReporter)
	reporter.Git2Chat = bugger.ghclient.Conf.Github2Chat

	for issue := range issueChan {
		reporter.addBug(issue)
	}


	return
}

func (bugger *Bugger) InitPlugin(bot *plotbot.Bot) {

	/*
	 * Get an array of issues matching Filters
	 */
	bugger.bot = bot

	var conf struct {
		Github github.Conf
	}

	bot.LoadConfig(&conf)

	bugger.ghclient = github.Client{
		Conf: conf.Github,
	}

	bot.ListenFor(&plotbot.Conversation{
		HandlerFunc: bugger.ChatHandler,
	})

}

func (bugger *Bugger) ChatHandler(conv *plotbot.Conversation, msg *plotbot.Message) {

	if !msg.MentionsMe {
		return
	}

	if msg.ContainsAny([]string{"bug report", "bug count"}) && msg.ContainsAny([]string{"how", "help"}) {

		var report string

		if msg.Contains("bug report") {
			report = "bug report"
		} else {
			report = "bug count"
		}
		mention := bugger.bot.MentionPrefix

		conv.Reply(msg, fmt.Sprintf(
			`*Usage:* %s [give me a | insert demand]  <%s>  [from the | syntax filler] [last | past] [n] [days | weeks]
*Examples:*
• %s please give me a %s over the last 5 days
• %s produce a %s   (7 day default)
• %s I want a %s from the past 2 weeks
• %s %s from the past week`, mention, report, mention, report, mention, report, mention, report, mention, report))

	} else if msg.Contains("bug report") {

		if len(bugger.ghclient.Conf.Repos) == 0 {
				log.Println("No repos configured - can't produce a bug report")
	    	return
	  }

		days := util.GetDaysFromQuery(msg.Text)
		bugger.messageReport(days, msg, conv, func() string {

			var reportsBuffer bytes.Buffer
			for repo in bugger.ghclient.Conf.Repos{
				reporter := bugger.makeBugReporter(days, repo)
				reportsBuffer.WriteString(reporter.printReport(days))
			}

			return reportsBuffer.String()

		})

	} else if msg.Contains("bug count") {

		if len(bugger.ghclient.Conf.Repos) == 0 {
				log.Println("No repos configured - can't produce a bug report")
	    	return
	  }

		days := util.GetDaysFromQuery(msg.Text)
		bugger.messageReport(days, msg, conv, func() string {

			var reportsBuffer bytes.Buffer
			for repo in bugger.ghclient.Conf.Repos{
				reporter := bugger.makeBugReporter(days, repo)
				reportsBuffer.WriteString(reporter.printCount(days))
			}

			return reportsBuffer.String()
		})

	}

	return

}

func (bugger *Bugger) messageReport(days int, msg *plotbot.Message, conv *plotbot.Conversation, genReport func() string) {

	if days > 31 {
		conv.Reply(msg, fmt.Sprintf("Whaoz, %d is too much data to compile - well maybe not, I am just scared", days))
		return
	}

	conv.Reply(msg, bugger.bot.WithMood("Building report - one moment please",
		"Whaooo! Pinging those githubbers - Let's do this!"))

	conv.Reply(msg, genReport())

}
