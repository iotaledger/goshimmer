package broadcast

import (
	"github.com/iotaledger/goshimmer/plugins/broadcast/server"
	"github.com/iotaledger/goshimmer/plugins/config"
	flag "github.com/spf13/pflag"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	pluginName = "Broadcast"
	bindAddress = "broadcast.bindAddress"
)

var (
	// plugin is the plugin instance of the activity plugin.
	plugin *node.Plugin
	once   sync.Once

	log *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(pluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Handler functions
func init() {
	flag.String(bindAddress, ":5050", "the bind address for the broadcast plugin")
}

// Configure events
func configure(_ *node.Plugin) {
	plugin.LogInfof("starting node with broadcast plugin")
	log = logger.NewLogger(pluginName)
}

//Run
func run(_ *node.Plugin) {

	//Server to connect to
	bindAddress := config.Node().String(bindAddress)
	log.Debugf("Starting Broadcast plugin on %s", bindAddress)
	err := daemon.BackgroundWorker("Broadcast worker", func(shutdownSignal <-chan struct{}) {
		err := server.Listen(bindAddress, log, shutdownSignal)
		if err != nil {
			log.Errorf("Failed to start Broadcast server: %v", err)
		}
		<-shutdownSignal
	})
	if err != nil {
		log.Errorf("Failed to start Broadcast daemon: %v", err)
	}


	//Get Messages from node
	notifyNewMsg := events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
			server.Broadcast(message.Bytes())
		})
	})

	if err := daemon.BackgroundWorker("Broadcast[MsgUpdater]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Storage.Events.MessageStored.Attach(notifyNewMsg)
		<-shutdownSignal
		log.Info("Stopping Broadcast...")
		messagelayer.Tangle().Storage.Events.MessageStored.Detach(notifyNewMsg)
		log.Info("Stopping Broadcast... \tDone")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
