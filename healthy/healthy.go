package healthy

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/plotly/plotbot"
)

type Healthy struct {
	urls []string
}

func init() {
	plotbot.RegisterPlugin(&Healthy{})
}

func (healthy *Healthy) InitPlugin(bot *plotbot.Bot) {
	var conf struct {
		Healthy struct {
			Urls []string
		}
	}
	bot.LoadConfig(&conf)

	healthy.urls = conf.Healthy.Urls

	bot.ListenFor(&plotbot.Conversation{
		MentionsMeOnly: true,
		ContainsAny:    []string{"health", "healthy?", "health check"},
		HandlerFunc:    healthy.HandleMessage,
	})
}

func (healthy *Healthy) HandleMessage(conv *plotbot.Conversation, msg *plotbot.Message) {
	success := []string{}
	failure := []string{}

	for _, url := range healthy.urls {
		if isHealthy(url) {
			success = append(success, url)
		} else {
			failure = append(failure, url)
		}
	}

	if len(success) > 0 {
		conv.Reply(msg, "All green for: "+
			strings.Join(success, ", "))
	}
	if len(failure) > 0 {
		conv.Reply(msg, "WARNING!! Something wrong with: "+
			strings.Join(failure, ", "))
	}
}

func isHealthy(url string) bool {
	res, err := http.Get(url)
	if err != nil {
		return false
	}

	defer res.Body.Close()
	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return false
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return false
	}

	return true
}
