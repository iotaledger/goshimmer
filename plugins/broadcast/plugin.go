package broadcast

import (
	"github.com/iotaledger/goshimmer/plugins/broadcast/server"
	"github.com/iotaledger/goshimmer/plugins/config"
	"sync"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	pluginName  = "Broadcast"
	bindAddress = "broadcast.bindAddress"
)

var (
	plugin *node.Plugin
	once   sync.Once

	log *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(pluginName, node.Disabled, configure, run)
	})
	return plugin
}

// ParametersDefinition contains the configuration parameters used by the plugin
type ParametersDefinition struct {
	Port int `default:"5050" usage:"port for broadcast plugin connections"`
}

var parameters = &ParametersDefinition{}

// Handler functions
func init() {
	configuration.BindParameters(parameters, "Broadcast")
}

// Configure events
func configure(_ *node.Plugin) {
	plugin.LogInfof("starting node with broadcast plugin")
}

//Run
func run(_ *node.Plugin) {

	//Server to connect to
	bindAddress := config.Node().String(bindAddress)
	plugin.LogInfof("Starting Broadcast plugin on %s", bindAddress)
	err := daemon.BackgroundWorker("Broadcast worker", func(shutdownSignal <-chan struct{}) {
		err := server.Listen(bindAddress, plugin, shutdownSignal)
		if err != nil {
			plugin.LogError("Failed to start Broadcast server: %v", err)
		}
		<-shutdownSignal
	})
	if err != nil {
		plugin.LogError("Failed to start Broadcast daemon: %v", err)
	}

	//Get Messages from node
	notifyNewMsg := events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
			go func() {
				server.Broadcast(message.Bytes())
			}()
		})
	})

	if err := daemon.BackgroundWorker("Broadcast[MsgUpdater]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Storage.Events.MessageStored.Attach(notifyNewMsg)
		<-shutdownSignal
		plugin.LogInfof("Stopping Broadcast...")
		messagelayer.Tangle().Storage.Events.MessageStored.Detach(notifyNewMsg)
		plugin.LogInfof("Stopping Broadcast... \tDone")
	}, shutdown.PriorityDashboard); err != nil {
		plugin.LogError("Failed to start as daemon: %s", err)
	}
}
