package totw

import "github.com/plotly/plotbot"

func init() {
	plotbot.RegisterPlugin(&Totw{})
}

type Totw struct {
	bot *plotbot.Bot
}

func (totw *Totw) InitPlugin(bot *plotbot.Bot) {
	plotbot.RegisterStringList("tech adept", []string{
		"You're a real tech adept",
		"What an investigator",
		"Such deep search!",
		"A real innovator that you are",
		"Way to go, I'm impressed!",
		"I'll take a look into this one",
		"You're generous!",
		"Hurray!",
	})

	totw.bot = bot

	bot.ListenFor(&plotbot.Conversation{
		HandlerFunc: totw.HandleMessage,
	})
}

func (totw *Totw) HandleMessage(conv *plotbot.Conversation, msg *plotbot.Message) {
	if msg.HasPrefix("!totw") || msg.HasPrefix("!techoftheweek") {
		conv.ReplyMention(msg, plotbot.RandomString("tech adept"))
	}
}
