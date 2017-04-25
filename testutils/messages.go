package testutils

import (
	"fmt"

	"github.com/nlopes/slack"
	"github.com/plotly/plotbot"
)

var DefaultFromUser = "hodor"

func applyFromUserToMessage(m *plotbot.Message, user string) {
	m.FromUser.Id = user
	m.FromUser.Name = user
	m.FromUser.RealName = user

	m.SubMessage.Username = user
	m.SubMessage.Id = user

	m.Username = user
	m.UserId = user
}

func ToBotMsg(bot plotbot.BotLike, msg string) *plotbot.Message {

	msg = fmt.Sprintf("@%s %s", bot.Id(), msg)
	channelId := "channelId"

	smsg := &slack.Msg{
		Id:        "abcdef",
		ChannelId: channelId,
		Text:      msg,
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
