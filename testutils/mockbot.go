package testutils

import (
	"fmt"
	"time"

	"github.com/nlopes/slack"
	"github.com/plotly/plotbot"
)

var BotId = "mockbotid"

type MockBot struct {
	Channels      map[string]slack.Channel
	Config        plotbot.SlackConfig
	MentionPrefix string
	Myself        *slack.UserDetails
	TestReplies   []*plotbot.BotReply
	TestNotifies  [][]string
	Users         map[string]slack.User
	conversations []*plotbot.Conversation
	mood          plotbot.Mood
}

func NewMockBot(sconf plotbot.SlackConfig, userconf slack.UserDetails, mood plotbot.Mood) *MockBot {

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
		Id:      "mockbot",
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

	bot := &MockBot{
		Channels:      make(map[string]slack.Channel),
		Config:        defaultsconf,
		MentionPrefix: fmt.Sprintf("@%s:", defaultsconf.Nickname),
		Myself:        &defaultuserconf,
		TestNotifies:  [][]string{},
		TestReplies:   []*plotbot.BotReply{},
		Users:         make(map[string]slack.User),
		mood:          mood,
	}

	return bot
}

func NewDefaultMockBot() *MockBot {
	return NewMockBot(plotbot.SlackConfig{}, slack.UserDetails{}, plotbot.Happy)
}

func ClearMockBot(bot *MockBot) {
	bot.TestNotifies = [][]string{}
	bot.TestReplies = []*plotbot.BotReply{}
}

func (bot *MockBot) LoadConfig(config interface{}) error {
	return nil
}

func (bot *MockBot) ListenFor(conv *plotbot.Conversation) error {
	return nil
}

func (bot *MockBot) Reply(msg *plotbot.Message, reply string) {
	bot.TestReplies = append(bot.TestReplies, msg.Reply(reply))
}

// ReplyMention replies with a @mention named prefixed, when replying in public. When replying in private, nothing is added.
func (bot *MockBot) ReplyMention(msg *plotbot.Message, reply string) {
	bot.Reply(msg, msg.AtMentionIfPublic(reply))
}

func (bot *MockBot) ReplyPrivately(msg *plotbot.Message, reply string) {
	bot.TestReplies = append(bot.TestReplies, msg.ReplyPrivately(reply))
}

func (bot *MockBot) Notify(room, color, msg string) {
	bot.TestNotifies = append(bot.TestNotifies, []string{room, color, msg})
}

func (bot *MockBot) SendToChannel(channelName string, message string) {
	reply := &plotbot.BotReply{
		To:   channelName,
		Text: message,
	}
	bot.TestReplies = append(bot.TestReplies, reply)
}

func (bot *MockBot) AtMention() string {
	return fmt.Sprintf("@%s:", bot.Myself.Name)
}

func (bot *MockBot) WithMood(happy string, hyper string) string {
	if bot.Mood() == plotbot.Happy {
		return happy
	} else {
		return hyper
	}
}

func (bot *MockBot) Mood() plotbot.Mood {
	return bot.mood
}

func (bot *MockBot) SetMood(mood plotbot.Mood) {
	bot.mood = mood
}

func (bot *MockBot) Id() string {
	return bot.Myself.Id
}

func (bot *MockBot) CloseConversation(conv *plotbot.Conversation) {
	for {
	}
}
