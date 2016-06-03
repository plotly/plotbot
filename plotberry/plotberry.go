package plotberry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/plotly/plotbot"
)

func init() {
	plotbot.RegisterPlugin(&PlotBerry{})
}

type PlotlyPlotBerryAnswer struct {
	Count int `json:"plotberries"`
}

type PlotBerry struct {
	bot        *plotbot.Bot
	totalUsers int
	config     PlotBerryConfig
}

type PlotBerryConfig struct {
	EraLength   int
	PingTime    time.Duration `json:"-"`
	RawPingTime int           `json:"PingTime"`
	Endpoint    string
}

func (pb *PlotBerry) InitPlugin(bot *plotbot.Bot) {
	var config struct {
		PlotBerry PlotBerryConfig
	}
	err := bot.LoadConfig(&config)
	if err != nil {
		log.Fatalln("plotberry: error loading config: ", err)
		return
	}

	pb.bot = bot
	pb.config = config.PlotBerry
	pb.config.PingTime = time.Duration(pb.config.RawPingTime) * time.Second

	if pb.config.EraLength == 0 {
		log.Fatal("plotberry: error: config 'EraLength' can't be 0, please configure it")
		return
	}

	statsChan := make(chan int, 100)
	go pb.goWatch(statsChan)
	go pb.goCount(statsChan)

	bot.ListenFor(&plotbot.Conversation{
		HandlerFunc: pb.handleMessage,
	})
}

func (pb *PlotBerry) handleMessage(conv *plotbot.Conversation, msg *plotbot.Message) {
	if msg.MentionsMe && msg.Contains("how many user") {
		conv.Reply(msg, fmt.Sprintf("We've got %d users!", pb.totalUsers))
	}
}

func (pb *PlotBerry) goWatch(ch chan int) {
	for {
		time.Sleep(pb.config.PingTime)

		count, err := getPlotBerry(pb.config.Endpoint)
		if err != nil {
			log.Println("plotberry: error fetching berries: ", err)
			continue
		}

		if count != pb.totalUsers {
			ch <- count
			pb.totalUsers = count
		}
	}
}

func (pb *PlotBerry) goCount(ch chan int) {
	send := func(msg string) {
		pb.bot.SendToChannel(pb.bot.Config.GeneralChannel, msg)
	}

	var lastCount int
	var countDownActive bool

	for count := range ch {
		untilNext := pb.config.EraLength - count%pb.config.EraLength
		nextEra := untilNext + count

		// we have already seen this count
		if lastCount == untilNext {
			continue
		}

		if untilNext <= 10 {
			// use a bool to handle the cases where we blow over the
			// era too quick
			countDownActive = true
		}

		switch untilNext {
		case 10:
			send(fmt.Sprintf("@all %d users till %d!", untilNext, nextEra))
		case 9:
			send(fmt.Sprintf("We're at %d users!", count))
		case 8:
			send(fmt.Sprintf("%d...", count))
		case 7:
			send(fmt.Sprintf("%d...\n", count))
		case 6:
			send(fmt.Sprintf("%d more to go", untilNext))
		case 5:
			send(fmt.Sprintf("@all %d users!\n I'm a freaky proud robot!", count))
		case 4:
			send(fmt.Sprintf("%d users till %d!", untilNext, nextEra))
		case 3:
			send(fmt.Sprintf("%d... \n", untilNext))
		case 2:
			send(fmt.Sprintf("%d more! swing badda badda badda badda badda.\n", untilNext))
		case 1:
			send("https://31.media.tumblr.com/3b74abfa367a3ed9a2cd753cd9018baa/tumblr_miul04oqog1qkp8xio1_400.gif")
			send(fmt.Sprintf("%d user until %d.\nYOU'RE ALL MAGIC!", untilNext, nextEra))
		case pb.config.EraLength: // use eraLength as 0
		default:
			if countDownActive {
				send(fmt.Sprintf("@all !!!\n We're at %d user signups!!!!! Whup Whup - Party for me this weekend", count))
				go func() {
					time.Sleep(22 * time.Second)
					send("...I like mimosas")
				}()

				countDownActive = false
			}
		}

		lastCount = untilNext
	}
}

func getPlotBerry(endpoint string) (int, error) {
	resp, err := http.Get(endpoint)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result PlotlyPlotBerryAnswer
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result.Count, err
}
