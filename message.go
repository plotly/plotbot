package plotbot

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/slack-go/slack"
)

type BotReply struct {
	To   string
	Text string
}

type Message struct {
	*slack.Msg
	SubMessage  *slack.Msg
	MentionsMe  bool
	IsEdition   bool
	FromMe      bool
	FromUser    *slack.User
	FromChannel *slack.Channel
}

func (msg *Message) IsPrivate() bool {
	return msg.Channel == ""
}

func (msg *Message) ContainsAnyCased(strs []string) bool {
	for _, s := range strs {
		if strings.Contains(msg.Msg.Text, s) {
			return true
		}
	}
	return false
}

func (msg *Message) ContainsAny(strs []string) bool {
	lowerStr := strings.ToLower(msg.Msg.Text)

	for _, s := range strs {
		lowerInput := strings.ToLower(s)

		if strings.Contains(lowerStr, lowerInput) {
			return true
		}
	}
	return false
}

func (msg *Message) ContainsAll(strs []string) bool {

	lowerStr := strings.ToLower(msg.Text)

	for _, s := range strs {
		lowerInput := strings.ToLower(s)

		if !strings.Contains(lowerStr, lowerInput) {
			return false
		}
	}
	return true
}

func (msg *Message) Contains(s string) bool {
	lowerStr := strings.ToLower(msg.Text)
	lowerInput := strings.ToLower(s)

	if strings.Contains(lowerStr, lowerInput) {
		return true
	}
	return false
}

func (msg *Message) HasPrefix(prefix string) bool {
	return strings.HasPrefix(msg.Text, prefix)
}

func (msg *Message) Reply(s string) *BotReply {
	rep := &BotReply{
		Text: s,
	}
	if msg.Channel != "" {
		rep.To = msg.Channel
	} else {
		rep.To = msg.Username
	}
	return rep
}

func (msg *Message) ReplyPrivately(s string) *BotReply {
	return &BotReply{
		To:   msg.Username,
		Text: s,
	}
}

func (msg *Message) String() string {
	return fmt.Sprintf("%#v", msg)
}

func (msg *Message) applyMentionsMe(bot *Bot) {
	if msg.IsPrivate() {
		msg.MentionsMe = true
	}

	m := reAtMention.FindStringSubmatch(msg.Text)
	if m != nil && m[1] == bot.Myself.ID {
		msg.MentionsMe = true
	}
}

func (msg *Message) applyFromMe(bot *Bot) {
	if msg.User == bot.Myself.ID {
		msg.FromMe = true
	}
}

func (msg *Message) AtMentionIfPublic(reply string) string {
	if msg.IsPrivate() {
		return reply
	} else {
		prefix := ""
		if msg.FromUser != nil {
			prefix = fmt.Sprintf("<@%s> ", msg.FromUser.Name)
		}
		return fmt.Sprintf("%s%s", prefix, reply)
	}
}

var reAtMention = regexp.MustCompile(`<@([A-Z0-9]+)(|([^>]+))>`)
