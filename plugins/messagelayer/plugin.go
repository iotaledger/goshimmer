package messagelayer

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName defines the plugin name.
	PluginName = "MessageLayer"
)

var (
	// plugin is the plugin instance of the message layer plugin.
	plugin           *node.Plugin
	pluginOnce       sync.Once
	messageRequester *tangle.MessageRequester
	msgReqOnce       sync.Once
	tangleInstance   *tangle.Tangle
	tangleOnce       sync.Once
	log              *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Tangle gets the tangle instance.
func Tangle() *tangle.Tangle {
	tangleOnce.Do(func() {
		tangleInstance = tangle.New(
			database.Store(),
			tangle.Identity(local.GetInstance().LocalIdentity()),
		)
	})
	return tangleInstance
}

// MessageRequester gets the messageRequester instance.
func MessageRequester() *tangle.MessageRequester {
	msgReqOnce.Do(func() {
		// load all missing messages on start up
		messageRequester = tangle.NewMessageRequester(Tangle().Storage.MissingMessages())
	})
	return messageRequester
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	// create instances
	messageRequester = MessageRequester()
	tangleInstance = Tangle()

	// setup messageRequester
	tangleInstance.Solidifier.Events.MessageMissing.Attach(events.NewClosure(messageRequester.StartRequest))
	tangleInstance.Storage.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()
		cachedMsgEvent.Message.Consume(func(msg *tangle.Message) {
			messageRequester.StopRequest(msg.ID())
		})
	}))

	MessageRequester().Events.MissingMessageAppeared.Attach(events.NewClosure(func(missingMessageAppeared *tangle.MissingMessageAppearedEvent) {
		tangleInstance.Storage.DeleteMissingMessage(missingMessageAppeared.ID)
	}))
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		tangleInstance.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
