package testutils

import (
	"fmt"
	"log"
	"time"

	"github.com/nlopes/slack"
	"github.com/plotly/plotbot"
)

type Bot struct {
	Config   plotbot.SlackConfig
	Users    map[string]slack.User
	Channels map[string]slack.Channel
	Myself   *slack.UserDetails

	// Internal handling
	conversations []*plotbot.Conversation
	MentionPrefix string

	Mood plotbot.Mood
}

func NewMockBot(sconf plotbot.SlackConfig, userconf slack.UserDetails, mood plotbot.Mood) *Bot {

	defaultsconf := plotbot.SlackConfig{
		Nickname: "mockbot",
		Username: "mockbot",
	}

	if sconf.Nickname != "" {
		defaultsconf.Nickname = sconf.Nickname
	}

	if sconf.Username != "" {
		defaultsconf.Username = sconf.Username
	}

	defaultuserconf := slack.UserDetails{
		Id:      "abcdefg",
		Name:    "mockbot",
		Created: slack.JSONTime(time.Now().Unix()),
	}

	if userconf.Id != "" {
		defaultuserconf.Id = userconf.Id
	}

	if userconf.Name != "" {
		defaultuserconf.Name = userconf.Name
	}

	if userconf.Created != 0 {
		defaultuserconf.Created = userconf.Created
	}

	bot := &Bot{
		Config:        defaultsconf,
		Users:         make(map[string]slack.User),
		Channels:      make(map[string]slack.Channel),
		MentionPrefix: fmt.Sprintf("@%s:", defaultsconf.Nickname),
		Mood:          mood,
		Myself:        &defaultuserconf,
	}

	return bot
}

func NewDefaultMockBot() *Bot {
	return NewMockBot(plotbot.SlackConfig{}, slack.UserDetails{}, plotbot.Happy)
}

func (bot *Bot) LoadConfig(config interface{}) error {
	return nil
}

func (bot *Bot) ListenFor(conv *plotbot.Conversation) error {
	return nil
}

func (bot *Bot) Reply(msg *plotbot.Message, reply string) {
	log.Println("Replying:", reply)
}

// ReplyMention replies with a @mention named prefixed, when replying in public. When replying in private, nothing is added.
func (bot *Bot) ReplyMention(msg *plotbot.Message, reply string) {
	log.Println("Replying:", reply)
}

func (bot *Bot) ReplyPrivately(msg *plotbot.Message, reply string) {
	log.Println("Replying privately:", reply)
}

func (bot *Bot) Notify(room, color, msg string) {
	log.Printf("Notify: %s\n", msg)
}

func (bot *Bot) SendToChannel(channelName string, message string) {
	log.Printf("Sending to channel %q: %q\n", channelName, message)
}

func (bot *Bot) Nickname() string {
	return bot.Config.Nickname
}

func (bot *Bot) AtMention() string {
	return fmt.Sprintf("@%s:", bot.Myself.Name)
}

func (bot *Bot) WithMood(happy string, hyper string) string {
	if bot.Mood == plotbot.Happy {
		return happy
	} else {
		return hyper
	}
}
