package funny

import (
	"time"

	"github.com/plotly/plotbot"
)

const replyLs = "```deploy/      Contributors-Guide/ image_server/     sheep_porn/     streambed/\nstreamhead/  README.md```"
const replyHardProblems = "naming things, cache invalidation and off-by-1 errors are the two most difficult computer science problems"
const replyInTheory = "yeah, theory and practice perfectly match... in theory."

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

func (funny *Funny) handleText(conv *plotbot.Conversation, msg *plotbot.Message) {
	if msg.Text == "ls" {
		conv.Reply(msg, replyLs)
	} else if msg.ContainsAny([]string{"difficult problem", "hard problem"}) {
		conv.Reply(msg, replyHardProblems)
	} else if msg.Contains("in theory") {
		conv.Reply(msg, replyInTheory)
	}
}

func (funny *Funny) handleJoke(conv *plotbot.Conversation, msg *plotbot.Message) {
	if conv.Bot.Mood == plotbot.Happy {
		conv.Reply(msg, "_blush_")
	} else {
		conv.Reply(msg, "here's another one")
		conv.Reply(msg, plotbot.RandomString("robot jokes"))
	}
}

func (funny *Funny) handleHowAreYou(conv *plotbot.Conversation, msg *plotbot.Message) {
	conv.ReplyMention(msg,
		conv.Bot.WithMood("Good, and you?", "I'm wild today!! wadabout you?"))

	conv.Bot.ListenFor(&plotbot.Conversation{
		ListenDuration: 60 * time.Second,
		WithUser:       msg.FromUser,
		InChannel:      msg.FromChannel,
		MentionsMeOnly: true,
		HandlerFunc: func(conv *plotbot.Conversation, msg *plotbot.Message) {
			conv.ReplyMention(msg,
				conv.Bot.WithMood("glad to hear it!", "zwweeeeeeeeet!"))
			conv.Close()
		},
		TimeoutFunc: func(conv *plotbot.Conversation) {
			conv.ReplyMention(msg, "well, we can catch up later")
		},
	})
}

func (funny *Funny) handleMention(conv *plotbot.Conversation, msg *plotbot.Message) {
	if msg.Contains("you're funny") {
		funny.handleJoke(conv, msg)
	} else if msg.ContainsAny([]string{"dumb ass", "dumbass"}) {
		conv.Reply(msg, "don't say such things")
	} else if msg.ContainsAny([]string{"thanks", "thank you", "thx", "thnks"}) {
		conv.Reply(msg, conv.Bot.WithMood("my pleasure", "any time, just ask, I'm here for you, ffiieeewww! get a life"))
	} else if msg.Contains("how are you") {
		funny.handleHowAreYou(conv, msg)
	}
}

func (funny *Funny) ChatHandler(conv *plotbot.Conversation, msg *plotbot.Message) {
	// Avoid matching on our own replies
	if msg.FromMe {
		return
	}

	if msg.MentionsMe {
		funny.handleMention(conv, msg)
	}

	funny.handleText(conv, msg)
}
