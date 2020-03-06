package transactionfactory

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/database"
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
	sequence *badger.Sequence
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)

	db := database.GetBadgerInstance()
	var err error
	sequence, err = db.GetSequence([]byte(DB_SEQUENCE_NUMBER), 100)
	if err != nil {
		log.Fatalf("Could not create transaction sequence number. %v", err)
	}
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PLUGIN_NAME, start, shutdown.ShutdownPriorityTransactionFactory); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Infof("Stopping %s ... done", PLUGIN_NAME)

	var n uint64
	n, _ = sequence.Next()
	fmt.Printf("#### Plugin configured! %d\n", n)

	<-shutdownSignal

	if err := sequence.Release(); err != nil {
		log.Errorf("Could not release transaction sequence number. %v", err)
	}

	log.Infof("Stopping %s ...", PLUGIN_NAME)
}
