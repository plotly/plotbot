package funny

import (
	"time"

	"github.com/plotly/plotbot"
)

type Funny struct {
}

func init() {
	plotbot.RegisterPlugin(&Funny{})
}

func (funny *Funny) InitPlugin(bot *plotbot.Bot) {

	bot.ListenFor(&plotbot.Conversation{
		HandlerFunc: funny.ChatHandler,
	})
}

func (funny *Funny) ChatHandler(conv *plotbot.Conversation, msg *plotbot.Message) {
	bot := conv.Bot
	if msg.MentionsMe {
		if msg.Contains("you're funny") {

			if bot.Mood == plotbot.Happy {
				conv.Reply(msg, "_blush_")
			} else {
				conv.Reply(msg, "here's another one")
				conv.Reply(msg, plotbot.RandomString("robot jokes"))
			}

		} else if msg.ContainsAny([]string{"dumb ass", "dumbass"}) {

			conv.Reply(msg, "don't say such things")

		} else if msg.ContainsAny([]string{"thanks", "thank you", "thx", "thnks"}) {
			conv.Reply(msg, bot.WithMood("my pleasure", "any time, just ask, I'm here for you, ffiieeewww!get a life"))

		} else if msg.Contains("how are you") && msg.MentionsMe {
			conv.ReplyMention(msg, bot.WithMood("good, and you ?", "I'm wild today!! wadabout you ?"))
			bot.ListenFor(&plotbot.Conversation{
				ListenDuration: 60 * time.Second,
				WithUser:       msg.FromUser,
				InChannel:      msg.FromChannel,
				MentionsMeOnly: true,
				HandlerFunc: func(conv *plotbot.Conversation, msg *plotbot.Message) {
					conv.ReplyMention(msg, bot.WithMood("glad to hear it!", "zwweeeeeeeeet !"))
					conv.Close()
				},
				TimeoutFunc: func(conv *plotbot.Conversation) {
					conv.ReplyMention(msg, "well, we can catch up later")
				},
			})
		}
	}

	if msg.Text == "ls" {

		conv.Reply(msg, "```deploy/      Contributors-Guide/ image_server/     sheep_porn/     streambed/\nstreamhead/  README.md```")

	} else if msg.ContainsAny([]string{"difficult problem", "hard problem"}) {

		conv.Reply(msg, "naming things, cache invalidation and off-by-1 errors are the two most difficult computer science problems")

	} else if msg.Contains("in theory") {

		conv.Reply(msg, "yeah, theory and practice perfectly match... in theory.")
	}

	return
}
