package zeromq

import (
	"strconv"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// zeromq logging is disabled by default
var PLUGIN = node.NewPlugin("ZeroMQ", node.Disabled, configure, run)
var log *logger.Logger
var publisher *Publisher
var emptyTag = strings.Repeat("9", 27)

// Configure the zeromq plugin
func configure(plugin *node.Plugin) {
	log = logger.NewLogger("ZeroMQ")
	tangle.Events.TransactionStored.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		// create goroutine for every event
		go func() {
			if err := publishTx(tx); err != nil {
				log.Errorf("error publishing tx: %s", err.Error())
			}
		}()
	}))
}

// Start the zeromq plugin
func run(plugin *node.Plugin) {
	zeromqPort := parameter.NodeConfig.GetInt(ZEROMQ_PORT)
	log.Infof("Starting ZeroMQ Publisher (port %d) ...", zeromqPort)

	daemon.BackgroundWorker("ZeroMQ Publisher", func(shutdownSignal <-chan struct{}) {
		if err := startPublisher(plugin); err != nil {
			log.Errorf("Stopping ZeroMQ Publisher: %s", err.Error())
		} else {
			log.Infof("Starting ZeroMQ Publisher (port %d) ... done", zeromqPort)
		}

		<-shutdownSignal

		log.Info("Stopping ZeroMQ Publisher ...")
		if err := publisher.Shutdown(); err != nil {
			log.Errorf("Stopping ZeroMQ Publisher: %s", err.Error())
		} else {
			log.Info("Stopping ZeroMQ Publisher ... done")
		}
	}, shutdown.ShutdownPriorityZMQ)
}

// Start the zmq publisher.
func startPublisher(plugin *node.Plugin) error {
	pub, err := NewPublisher()
	if err != nil {
		return err
	}
	publisher = pub

	return publisher.Start(parameter.NodeConfig.GetInt(ZEROMQ_PORT))
}

// Publish a transaction that has recently been added to the ledger
func publishTx(tx *value_transaction.ValueTransaction) error {

	hash := tx.MetaTransaction.GetHash()
	address := tx.GetAddress()
	value := tx.GetValue()
	timestamp := int64(tx.GetTimestamp())
	trunk := tx.MetaTransaction.GetTrunkTransactionHash()
	branch := tx.MetaTransaction.GetBranchTransactionHash()
	stored := time.Now().Unix()

	messages := []string{
		"tx",                             // ZMQ event
		hash,                             // Transaction hash
		address,                          // Address
		strconv.FormatInt(value, 10),     // Value
		emptyTag,                         // Obsolete tag
		strconv.FormatInt(timestamp, 10), // Timestamp
		"0",                              // Index of the transaction in the bundle
		"0",                              // Last transaction index of the bundle
		hash,                             // Bundle hash
		trunk,                            // Trunk transaction hash
		branch,                           // Branch transaction hash
		strconv.FormatInt(stored, 10),    // Unix timestamp for when the transaction was received
		emptyTag,                         // Tag
	}

	return publisher.Send(messages)
}
