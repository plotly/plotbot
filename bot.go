package plotbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/plotly/plotbot/Godeps/_workspace/src/github.com/nlopes/slack"
	"github.com/plotly/plotbot/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb"
)

type Bot struct {
	// Global bot configuration
	configFile string
	Config     SlackConfig

	// Slack connectivity
	Slack    *slack.Slack
	ws       *slack.SlackWS
	Users    map[string]slack.User
	Channels map[string]slack.Channel
	Myself   *slack.UserDetails

	// Internal handling
	conversations     []*Conversation
	addConversationCh chan *Conversation
	delConversationCh chan *Conversation
	disconnected      chan bool
	replySink         chan *BotReply
	MentionPrefix     string

	// Storage
	LevelDBConfig LevelDBConfig
	DB            *leveldb.DB

	// Other features
	WebServer WebServer
	Mood      Mood
}

func New(configFile string) *Bot {
	bot := &Bot{
		configFile:        configFile,
		replySink:         make(chan *BotReply, 10),
		addConversationCh: make(chan *Conversation, 100),
		delConversationCh: make(chan *Conversation, 100),

		Users:    make(map[string]slack.User),
		Channels: make(map[string]slack.Channel),
	}

	return bot
}

func (bot *Bot) Run() {
	bot.loadBaseConfig()

	// Write PID
	err := bot.writePID()
	if err != nil {
		log.Fatal("Couldn't write PID file:", err)
	}

	db, err := leveldb.OpenFile(bot.LevelDBConfig.Path, nil)
	if err != nil {
		log.Fatal("Could not initialize Leveldb key/value store:", err)
	}
	defer func() {
		log.Fatal("Database is closing")
		db.Close()
	}()
	bot.DB = db

	// Init all plugins
	enabledPlugins := make([]string, 0)
	for _, plugin := range registeredPlugins {
		pluginType := reflect.TypeOf(plugin)
		if pluginType.Kind() == reflect.Ptr {
			pluginType = pluginType.Elem()
		}
		typeList := make([]string, 0)
		if _, ok := plugin.(PluginInitializer); ok {
			typeList = append(typeList, "Plugin")
		}
		if _, ok := plugin.(WebServer); ok {
			typeList = append(typeList, "WebServer")
		}
		if _, ok := plugin.(WebServerAuth); ok {
			typeList = append(typeList, "WebServerAuth")
		}
		if _, ok := plugin.(WebPlugin); ok {
			typeList = append(typeList, "WebPlugin")
		}

		log.Printf("Plugin %s implements %s", pluginType.String(),
			strings.Join(typeList, ", "))
		enabledPlugins = append(enabledPlugins, strings.Replace(pluginType.String(), ".", "_", -1))
	}

	initChatPlugins(bot)
	initWebServer(bot, enabledPlugins)
	initWebPlugins(bot)

	if bot.WebServer != nil {
		go bot.WebServer.RunServer()
	}

	for {
		log.Println("Connecting client...")
		err := bot.connectClient()
		if err != nil {
			log.Println("Error in connectClient(): ", err)
			time.Sleep(3 * time.Second)
			continue
		}

		bot.MentionPrefix = fmt.Sprintf("@%s:", bot.Myself.Name)

		bot.setupHandlers()

		select {
		case <-bot.disconnected:
			log.Println("Disconnected...")
			time.Sleep(1 * time.Second)
			continue
		}
	}
}

func (bot *Bot) writePID() error {
	var serverConf struct {
		Server struct {
			Pidfile string `json:"pid_file"`
		}
	}

	err := bot.LoadConfig(&serverConf)
	if err != nil {
		return err
	}

	if serverConf.Server.Pidfile == "" {
		return nil
	}

	pid := os.Getpid()
	pidb := []byte(strconv.Itoa(pid))
	return ioutil.WriteFile(serverConf.Server.Pidfile, pidb, 0755)
}

func (bot *Bot) ListenFor(conv *Conversation) error {
	conv.Bot = bot

	err := conv.checkParams()
	if err != nil {
		log.Println("Bot.ListenFor(): Invalid Conversation: ", err)
		return err
	}

	conv.setupChannels()

	if conv.isManaged() {
		go conv.launchManager()
	}

	bot.addConversationCh <- conv

	return nil
}

func (bot *Bot) Reply(msg *Message, reply string) {
	log.Println("Replying:", reply)
	bot.replySink <- msg.Reply(reply)
}

// ReplyMention replies with a @mention named prefixed, when replying in public. When replying in private, nothing is added.
func (bot *Bot) ReplyMention(msg *Message, reply string) {
	if msg.IsPrivate() {
		bot.Reply(msg, reply)
	} else {
		prefix := ""
		if msg.FromUser != nil {
			prefix = fmt.Sprintf("<@%s> ", msg.FromUser.Name)
		}
		bot.Reply(msg, fmt.Sprintf("%s%s", prefix, reply))
	}
}

func (bot *Bot) ReplyPrivately(msg *Message, reply string) {
	log.Println("Replying privately:", reply)
	bot.replySink <- msg.ReplyPrivately(reply)
}

func (bot *Bot) Notify(room, color, msg string) {
	_, _, err := bot.Slack.PostMessage(room, "", slack.PostMessageParameters{
		Attachments: []slack.Attachment{
			{
				Color: color,
				Text:  msg,
			},
		},
	})

	if err != nil {
		log.Printf("Notify error: %s\n", err)
	}
}

func (bot *Bot) SendToChannel(channelName string, message string) {
	channel := bot.GetChannelByName(channelName)

	if channel == nil {
		log.Printf("Couldn't send message, channel %q not found: %q\n", channelName, message)
		return
	}
	log.Printf("Sending to channel %q: %q\n", channelName, message)

	reply := &BotReply{
		To:   channel.Id,
		Text: message,
	}
	bot.replySink <- reply
	return
}

func (bot *Bot) connectClient() (err error) {
	resource := bot.Config.Resource
	if resource == "" {
		resource = "bot"
	}

	bot.Slack = slack.New(bot.Config.ApiToken)
	//bot.Slack.SetDebug(true)

	ws, err := bot.Slack.StartRTM("", "http://safeidentity.slack.com")
	if err != nil {
		return err
	}
	bot.ws = ws

	infos := bot.Slack.GetInfo()
	bot.Myself = infos.User
	bot.cacheUsers(infos.Users)
	bot.cacheChannels(infos.Channels, infos.Groups)

	return
}

func (bot *Bot) setupHandlers() {
	bot.disconnected = make(chan bool)
	go keepaliveSlackWS(bot.ws)
	go bot.replyHandler()
	go bot.messageHandler()
	log.Println("Bot ready")
}

func (bot *Bot) cacheUsers(users []slack.User) {
	bot.Users = make(map[string]slack.User)
	for _, user := range users {
		bot.Users[user.Id] = user
	}
}

func (bot *Bot) cacheChannels(channels []slack.Channel, groups []slack.Group) {
	bot.Channels = make(map[string]slack.Channel)
	for _, channel := range channels {
		bot.Channels[channel.Id] = channel
	}

	for _, group := range groups {
		bot.Channels[group.Id] = slack.Channel{
			BaseChannel: group.BaseChannel,
			Name:        group.Name,
			IsChannel:   false,
			Creator:     group.Creator,
			IsArchived:  group.IsArchived,
			Members:     group.Members,
			Topic:       group.Topic,
			Purpose:     group.Purpose,
			IsMember:    true,
			NumMembers:  group.NumMembers,
		}
	}
}

func keepaliveSlackWS(ws *slack.SlackWS) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		if err := ws.Ping(); err != nil {
			return
		}
	}
}

func (bot *Bot) loadBaseConfig() {
	if err := checkPermission(bot.configFile); err != nil {
		log.Fatal("ERROR Checking Permissions: ", err)
	}

	var config1 struct {
		Slack SlackConfig
	}
	err := bot.LoadConfig(&config1)
	if err != nil {
		log.Fatalln("Error loading Slack config section:", err)
	} else {
		bot.Config = config1.Slack
	}

	var config2 struct {
		LevelDB LevelDBConfig
	}
	err = bot.LoadConfig(&config2)
	if err != nil {
		log.Fatalln("Error loading LevelDB config section:", err)
	} else {
		bot.LevelDBConfig = config2.LevelDB
	}
}

func (bot *Bot) LoadConfig(config interface{}) (err error) {
	content, err := ioutil.ReadFile(bot.configFile)
	if err != nil {
		log.Fatalln("LoadConfig(): Error reading config:", err)
		return
	}
	err = json.Unmarshal(content, &config)

	if err != nil {
		log.Println("LoadConfig(): Error unmarshaling JSON", err)
	}
	return
}

func (bot *Bot) replyHandler() {
	count := 0
	for {
		select {
		case <-bot.disconnected:
			return
		case reply := <-bot.replySink:
			if reply != nil {
				//log.Println("REPLYING", reply.To, reply.Message)
				count += 1
				err := bot.ws.SendMessage(&slack.OutgoingMessage{
					Id:        count,
					ChannelId: reply.To,
					Type:      "message",
					Text:      reply.Text,
				})
				if err != nil {
					return
				}
				time.Sleep(50 * time.Millisecond)

			}
		}
	}
}

func (bot *Bot) removeConversation(conv *Conversation) {
	for i, element := range bot.conversations {
		if element == conv {
			// following: https://code.google.com/p/go-wiki/wiki/SliceTricks
			copy(bot.conversations[i:], bot.conversations[i+1:])
			bot.conversations[len(bot.conversations)-1] = nil
			bot.conversations = bot.conversations[:len(bot.conversations)-1]
			return
		}
	}

	return
}

func (bot *Bot) messageHandler() {
	events := make(chan slack.SlackEvent, 10)
	go bot.ws.HandleIncomingEvents(events)

	for {
		select {
		case <-bot.disconnected:
			return

		case conv := <-bot.addConversationCh:
			bot.conversations = append(bot.conversations, conv)

		case conv := <-bot.delConversationCh:
			bot.removeConversation(conv)

		case event := <-events:
			bot.handleRTMEvent(&event)
		}

		// Always flush conversations deletions between messages, so a
		// Close()'d Conversation never processes another message.
		select {
		case conv := <-bot.delConversationCh:
			bot.removeConversation(conv)
		default:
		}
	}
}

func (bot *Bot) handleRTMEvent(event *slack.SlackEvent) {
	switch ev := event.Data.(type) {
	case slack.HelloEvent:
		fmt.Println("Got a HELLO from websocket")

	case *slack.MessageEvent:
		fmt.Printf("Message: %v\n", ev)
		msg := &Message{
			Msg:        &ev.Msg,
			SubMessage: ev.SubMessage,
		}

		user, ok := bot.Users[ev.UserId]
		if ok {
			msg.FromUser = &user
		}
		channel, ok := bot.Channels[ev.ChannelId]
		if ok {
			msg.FromChannel = &channel
		}

		msg.applyMentionsMe(bot)
		msg.applyFromMe(bot)

		log.Printf("Incoming message: %s\n", msg)

		for _, conv := range bot.conversations {
			filterFunc := defaultFilterFunc
			if conv.FilterFunc != nil {
				filterFunc = conv.FilterFunc
			}

			if filterFunc(conv, msg) {
				conv.HandlerFunc(conv, msg)
			}
		}

	case *slack.PresenceChangeEvent:
		user := bot.Users[ev.UserId]
		log.Printf("User %q is now %q\n", user.Name, ev.Presence)
		user.Presence = ev.Presence

	case slack.LatencyReport:
		break
	case *slack.SlackWSError:
		fmt.Printf("Error: %d - %s\n", ev.Code, ev.Msg)

	// TODO: manage im_open, im_close, and im_created ?

	/**
	 * User changes
	 */
	case *slack.UserChangeEvent:
		bot.Users[ev.User.Id] = ev.User

	/**
	 * Handle channel changes
	 */
	case *slack.ChannelRenameEvent:
		channel := bot.Channels[ev.Channel.Id]
		channel.Name = ev.Channel.Name

	case *slack.ChannelJoinedEvent:
		bot.Channels[ev.Channel.Id] = ev.Channel

	case *slack.ChannelCreatedEvent:
		bot.Channels[ev.Channel.Id] = slack.Channel{
			BaseChannel: slack.BaseChannel{
				Id: ev.Channel.Id,
			},
			Name:    ev.Channel.Name,
			Creator: ev.Channel.Creator,
		}
		// NICE: poll the API to get a full Channel object ? many
		// things are missing here

	case *slack.ChannelDeletedEvent:
		delete(bot.Channels, ev.ChannelId)

	case *slack.ChannelArchiveEvent:
		channel := bot.Channels[ev.ChannelId]
		channel.IsArchived = true

	case *slack.ChannelUnarchiveEvent:
		channel := bot.Channels[ev.ChannelId]
		channel.IsArchived = false

	/**
	 * Handle group changes
	 */
	case *slack.GroupRenameEvent:
		group := bot.Channels[ev.Channel.Id]
		group.Name = ev.Channel.Name

	case *slack.GroupJoinedEvent:
		bot.Channels[ev.Channel.Id] = ev.Channel

	case *slack.GroupCreatedEvent:
		bot.Channels[ev.Channel.Id] = slack.Channel{
			BaseChannel: slack.BaseChannel{
				Id: ev.Channel.Id,
			},
			Name:    ev.Channel.Name,
			Creator: ev.Channel.Creator,
		}
		// NICE: poll the API to get a full Group object ? many
		// things are missing here

	case *slack.GroupCloseEvent:
		// TODO: when a group is "closed"... does that mean removed ?
		// TODO: how do we even manage groups ?!?!
		delete(bot.Channels, ev.ChannelId)

	case *slack.GroupArchiveEvent:
		group := bot.Channels[ev.ChannelId]
		group.IsArchived = true

	case *slack.GroupUnarchiveEvent:
		group := bot.Channels[ev.ChannelId]
		group.IsArchived = false

	default:
		fmt.Printf("Unexpected: %v\n", ev)
	}
}

// Disconnect, you can call many times, checks closed channel first.
func (bot *Bot) Disconnect() {
	select {
	case _, ok := <-bot.disconnected:
		if ok {
			close(bot.disconnected)
		}
	default:
	}
}

// GetUser returns a *slack.User by ID, Name, RealName or Email
func (bot *Bot) GetUser(find string) *slack.User {
	for _, user := range bot.Users {
		//log.Printf("Hmmmm, %#v\n", user)
		if user.Profile.Email == find || user.Id == find || user.Name == find || user.RealName == find {
			return &user
		}
	}
	return nil
}

// GetChannelByName returns a *slack.Channel by Name
func (bot *Bot) GetChannelByName(name string) *slack.Channel {
	name = strings.TrimLeft(name, "#")
	for _, channel := range bot.Channels {
		if channel.Name == name {
			return &channel
		}
	}
	return nil
}
