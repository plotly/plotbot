package testutils

import (
	"fmt"

	"github.com/slack-go/slack"
	"github.com/plotly/plotbot"
)

var DefaultFromUser = "hodor"

func applyFromUserToMessage(m *plotbot.Message, user string) {
	m.FromUser.ID = user
	m.FromUser.Name = user
	m.FromUser.RealName = user

	m.SubMessage.User = user

	m.Username = user
	m.User = user
}

func ToBotMsg(bot plotbot.BotLike, msg string) *plotbot.Message {

	msg = fmt.Sprintf("@%s %s", bot.Id(), msg)
	channelId := "channelId"

	smsg := &slack.Msg{
		User:    "abcdef",
		Channel: channelId,
		Text:    msg,
	}

	suser := &slack.User{}

	m := &plotbot.Message{
		Msg:        smsg,
		SubMessage: smsg,
		FromUser:   suser,
		MentionsMe: true,
		FromMe:     false,
	}
	applyFromUserToMessage(m, DefaultFromUser)
	return m
}

func ToBotMsgFromUser(bot plotbot.BotLike, msg, user string) *plotbot.Message {
	m := ToBotMsg(bot, msg)
	applyFromUserToMessage(m, user)
	return m
}
