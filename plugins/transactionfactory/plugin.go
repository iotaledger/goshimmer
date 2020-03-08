package transactionfactory

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/transactionfactory"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
)

const (
	PLUGIN_NAME        = "TransactionFactory"
	DB_SEQUENCE_NUMBER = "seq"
)

var (
	PLUGIN   = node.NewPlugin(PLUGIN_NAME, node.Enabled, configure, run)
	log      *logger.Logger
	instance *transactionfactory.TransactionFactory
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)

	instance = transactionfactory.Setup(log, database.GetBadgerInstance(), []byte(DB_SEQUENCE_NUMBER))

	// configure events
	//transactionfactory.Events.PayloadConstructed.Attach(events.NewClosure(func(payload *payload.Payload) {
	//	instance.BuildTransaction(payload)
	//}))

	transactionfactory.Events.TransactionConstructed.Attach(events.NewClosure(func(tx *transaction.Transaction) {
		fmt.Printf("Transaction created: %v\n", tx)
		//	TODO: call gossip
	}))
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PLUGIN_NAME, start, shutdown.ShutdownPriorityTransactionFactory); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Infof("Stopping %s ... done", PLUGIN_NAME)

	<-shutdownSignal

	instance.Shutdown()

	log.Infof("Stopping %s ...", PLUGIN_NAME)
}
