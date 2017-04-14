package testutils

import (
	"fmt"

	"github.com/nlopes/slack"
	"github.com/plotly/plotbot"
)

func ToBotMsg(bot plotbot.BotLike, msg string) plotbot.Message {

	msg = fmt.Sprintf("@%s %s", bot.Id(), msg)

	userId := "userId"
	fromUser := "hodor"
	channelId := "channelId"

	smsg := &slack.Msg{
		Id:        "abcdef",
		UserId:    userId,
		Username:  fromUser,
		ChannelId: channelId,
		Text:      msg,
	}

	suser := &slack.User{
		Id:   userId,
		Name: fromUser,
	}

	return plotbot.Message{
		Msg:        smsg,
		SubMessage: smsg,
		FromUser:   suser,
		MentionsMe: true,
		FromMe:     false,
	}
}
