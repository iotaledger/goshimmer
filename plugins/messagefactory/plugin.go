package messagefactory

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/messsagefactory"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
)

const (
	PLUGIN_NAME        = "MessageFactory"
	DB_SEQUENCE_NUMBER = "seq"
)

var (
	PLUGIN   = node.NewPlugin(PLUGIN_NAME, node.Enabled, configure, run)
	log      *logger.Logger
	instance *messsagefactory.MessageFactory
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)

	instance = messsagefactory.Setup(
		log,
		database.GetBadgerInstance(),
		local.GetInstance().LocalIdentity(),
		tangle.TipSelector,
		[]byte(DB_SEQUENCE_NUMBER),
	)

	// configure events
	//messsagefactory.Events.PayloadConstructed.Attach(events.NewClosure(func(payload *payload.Payload) {
	//	instance.BuildMessage(payload)
	//}))

	messsagefactory.Events.MessageConstructed.Attach(events.NewClosure(func(msg *message.Transaction) {
		fmt.Printf("Message created: %v\n", msg)
		//	TODO: call gossip
	}))
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PLUGIN_NAME, start, shutdown.ShutdownPriorityMessageFactory); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Infof("Stopping %s ... done", PLUGIN_NAME)

	<-shutdownSignal

	instance.Shutdown()

	log.Infof("Stopping %s ...", PLUGIN_NAME)
}
