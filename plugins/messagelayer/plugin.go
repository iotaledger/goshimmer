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
	plugin         *node.Plugin
	pluginOnce     sync.Once
	tangleInstance *tangle.Tangle
	tangleOnce     sync.Once
	log            *logger.Logger
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
			tangle.Store(database.Store()),
			tangle.Identity(local.GetInstance().LocalIdentity()),
			tangle.WithoutOpinionFormer(true),
		)
	})

	return tangleInstance
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	Tangle().Setup()

	// Bypass the Booker
	Tangle().Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			messageMetadata.SetBooked(true)
			Tangle().Booker.Events.MessageBooked.Trigger(messageID)
		})
	}))

	Tangle().Events.Error.Attach(events.NewClosure(func(err error) {
		log.Error(err)
	}))
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		Tangle().Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
