package plotberry

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/plotly/plotbot"
)

type PlotBerry struct {
	bot        *plotbot.Bot
	totalUsers int
	pingTime   time.Duration
	era        int
	endpoint   string
}

type TotalUsers struct {
	Plotberries int `json:"plotberries"`
}

type PlotberryConf struct {
	Era      int
	PingTime int
	EndPoint string
}

func init() {
	plotbot.RegisterPlugin(&PlotBerry{})
}

func (plotberry *PlotBerry) InitChatPlugin(bot *plotbot.Bot) {

	var conf struct {
		Plotberry PlotberryConf
	}
	err := bot.LoadConfig(&conf)
	if err != nil {
		log.Fatalln("Error loading PlotBerry config section: ", err)
		return
	}

	plotberry.bot = bot
	plotberry.pingTime = time.Duration(conf.Plotberry.PingTime) * time.Second
	plotberry.era = conf.Plotberry.Era
	plotberry.endpoint = conf.Plotberry.EndPoint

	// if plotberry.era is 0 we will get a divide by zero error - lets abort first
	if plotberry.era == 0 {
		log.Fatal("Plotberry.era may not be zero (divide by zero error), please set the configuration")
		return
	}

	statchan := make(chan TotalUsers, 100)

	go plotberry.launchWatcher(statchan)
	go plotberry.launchCounter(statchan)

	bot.ListenFor(&plotbot.Conversation{
		HandlerFunc: plotberry.ChatHandler,
	})
}

func (plotberry *PlotBerry) ChatHandler(conv *plotbot.Conversation, msg *plotbot.Message) {
	if msg.MentionsMe && msg.Contains("how many user") {
		conv.Reply(msg, fmt.Sprintf("We got %d users!", plotberry.totalUsers))
	}
	return
}

func getplotberry(endpoint string) (*TotalUsers, error) {

	var data TotalUsers

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (plotberry *PlotBerry) launchWatcher(statchan chan TotalUsers) {

	for {
		time.Sleep(plotberry.pingTime)

		data, err := getplotberry(plotberry.endpoint)

		if err != nil {
			log.Print(err)
			continue
		}

		if data.Plotberries != plotberry.totalUsers {
			statchan <- *data
		}

		plotberry.totalUsers = data.Plotberries
	}
}

func (plotberry *PlotBerry) launchCounter(statchan chan TotalUsers) {

	var lastCount int
	var countDownActive bool

	send := func(msg string) {
		plotberry.bot.SendToRoom(plotberry.bot.Config.TeamRoom, msg)
	}

	doFinale := func(msg string) {
		send(msg)
		go func() {
			time.Sleep(22 * time.Second)
			plotberry.bot.SendToRoom(plotberry.bot.Config.TeamRoom, "...I like mimosas")
		}()
	}

	for data := range statchan {

		totalUsers := data.Plotberries
		untilNext := plotberry.era - totalUsers%plotberry.era
		nextEra := untilNext + totalUsers

		// we have already seen this count
		if lastCount == untilNext {
			continue
		}

		if untilNext <= 10 {
			countDownActive = true
		}

		switch untilNext {

		case 10:
			send(fmt.Sprintf("@all %d users till %d!", untilNext, nextEra))
		case 9:
			send(fmt.Sprintf("We're at %d users!", totalUsers))
		case 8:
			send(fmt.Sprintf("%d...", totalUsers))
		case 7:
			send(fmt.Sprintf("%d...\n", totalUsers))
		case 6:
			send(fmt.Sprintf("%d more to go", untilNext))
		case 5:
			send(fmt.Sprintf("@all %d users!\n I'm a freaky proud robot!", totalUsers))
		case 4:
			send(fmt.Sprintf("%d users till %d!", untilNext, nextEra))
		case 3:
			send(fmt.Sprintf("%d... \n", untilNext))
		case 2:
			send(fmt.Sprintf("%d more! swing badda badda badda badda badda.\n", untilNext))
		case 1:
			send("https://31.media.tumblr.com/3b74abfa367a3ed9a2cd753cd9018baa/tumblr_miul04oqog1qkp8xio1_400.gif")
			send(fmt.Sprintf("%d user until %d.\nYOU'RE ALL MAGIC!", untilNext, nextEra))

			// use plotberry era as untilNext will == plotbot.era when totalUsers mod era == 0
		case plotberry.era:
			doFinale(fmt.Sprintf("@all !!!\n We're at %d user signups!!!!! Whup Whup - Party for me this weekend", totalUsers))
			countDownActive = false
		default:
			// too many users signed on within the ping time and we blew past our era. Play the finale
			if countDownActive {
				doFinale(fmt.Sprintf("@all !!!\n We're at %d user signups!!!!! Whup Whup - Party for me this weekend", totalUsers))
				countDownActive = false
			}
		}

		lastCount = untilNext
	}

}
