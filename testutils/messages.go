package testutils

import (
	"fmt"

	"github.com/nlopes/slack"
	"github.com/plotly/plotbot"
)

var DefaultFromUser = "hodor"

func applyFromUserToMessage(m plotbot.Message, user string) plotbot.Message {
	m.FromUser.Id = user
	m.FromUser.Name = user
	m.SubMessage.Username = user
	m.SubMessage.Id = user
	m.Username = user
	m.UserId = user

	return m
}

func ToBotMsg(bot plotbot.BotLike, msg string) plotbot.Message {

	msg = fmt.Sprintf("@%s %s", bot.Id(), msg)

	channelId := "channelId"

	smsg := &slack.Msg{
		Id:        "abcdef",
		UserId:    "",
		Username:  "",
		ChannelId: channelId,
		Text:      msg,
	}

	suser := &slack.User{
		Id:   "",
		Name: "",
	}

	return applyFromUserToMessage(plotbot.Message{
		Msg:        smsg,
		SubMessage: smsg,
		FromUser:   suser,
		MentionsMe: true,
		FromMe:     false,
	}, DefaultFromUser)
}

func ToBotMsgFromUser(bot plotbot.BotLike, msg, user string) plotbot.Message {
	return applyFromUserToMessage(ToBotMsg(bot, msg), user)
}
