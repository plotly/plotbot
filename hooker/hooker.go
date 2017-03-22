package hooker

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/plotly/plotbot"
)

type Hooker struct {
	bot    *plotbot.Bot
	config HookerConfig
}

func init() {
	plotbot.RegisterPlugin(&Hooker{})
}

type HookerConfig struct {
	StripeSecret string
	GitHubSecret string
}

type MonitAlert struct {
	Host    string `json:"host"`
	Date    string `json:"date"`
	Service string `json:"service"`
	Alert   string `json:"alert"`
}

func (hooker *Hooker) InitWebPlugin(
	bot *plotbot.Bot, privateRouter *mux.Router, publicRouter *mux.Router,
) {
	var conf struct {
		Hooker HookerConfig
	}
	bot.LoadConfig(&conf)
	hooker.config = conf.Hooker
	hooker.bot = bot

	publicRouter.HandleFunc(
		"/public/hooks/monit",
		hooker.handleMonitHook,
	)

	publicRouter.HandleFunc(
		"/public/hooks/github",
		hooker.handleGithubHook,
	)
}

func (hooker *Hooker) handleMonitHook(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != "POST" {
		sendMethodNotAllowed(w)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("hooker: monit hook: error reading body: ", err)
		return
	}

	var alert MonitAlert
	err = json.Unmarshal(body, &alert)
	if err != nil {
		log.Println("hooker: monit hook: error parsing json:", err)
		return
	}

	// TODO Do something with monit alert
}

func (hooker *Hooker) handleGithubHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendMethodNotAllowed(w)
		return
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		log.Println("hooker: github hook: error reading body: ", err)
		return
	}

	var payload map[string]interface{}
	err = json.Unmarshal(body, &payload)
	if err != nil {
		log.Println("hooker: github hook: error parsing json:", err)
		return
	}

	// TODO Do something with GitHub hook
}

func sendMethodNotAllowed(w http.ResponseWriter) {
	http.Error(w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed)
}
