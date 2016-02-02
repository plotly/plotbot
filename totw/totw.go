package totw

import (
	"strings"

	"github.com/plotly/plotbot"
)

type Totw struct {
	bot *plotbot.Bot
}

func init() {
	plotbot.RegisterPlugin(&Totw{})
}

func (totw *Totw) InitPlugin(bot *plotbot.Bot) {
	plotbot.RegisterStringList("tech adept", []string{
		"you're a real tech adept",
		"what an investigator",
		"such deep search!",
		"a real innovator you are",
		"way to go, I'm impressed",
		"I'll take a look into this one",
		"you're generous!",
		"hurray!",
	})

	totw.bot = bot

	bot.ListenFor(&plotbot.Conversation{
		HandlerFunc: totw.ChatHandler,
	})
}

func (totw *Totw) ChatHandler(conv *plotbot.Conversation, msg *plotbot.Message) {
	if strings.HasPrefix(msg.Text, "!totw") || strings.HasPrefix(msg.Text, "!techoftheweek") {
		conv.ReplyMention(msg, plotbot.RandomString("tech adept"))
	}
}
