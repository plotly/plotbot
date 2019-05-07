package main

import (
	"flag"
	"os"

	"github.com/plotly/plotbot"
	_ "github.com/plotly/plotbot/bugger"
	_ "github.com/plotly/plotbot/deployer"
	_ "github.com/plotly/plotbot/mooder"
	_ "github.com/plotly/plotbot/plotberry"
)

var configFile = flag.String("config", os.Getenv("HOME")+"/.plotbot", "config file")

func main() {
	flag.Parse()

	bot := plotbot.New(*configFile)

	bot.Run()
}
