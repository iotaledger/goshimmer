package zeromq

import (
	"strconv"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("ZeroMQ", configure, run)

var publisher *Publisher
var emptyTag = strings.Repeat("9", 27)

// Configure the zeromq plugin
func configure(plugin *node.Plugin) {

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		plugin.LogInfo("Stopping ZeroMQ Publisher ...")

		if err := publisher.Shutdown(); err != nil {
			plugin.LogFailure("Stopping ZeroMQ Publisher: " + err.Error())
		} else {
			plugin.LogSuccess("Stopping ZeroMQ Publisher ... done")
		}
	}))

	tangle.Events.TransactionStored.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		// create goroutine for every event
		go func() {
			if err := publishTx(tx); err != nil {
				plugin.LogFailure(err.Error())
			}
		}()
	}))
}

// Start the zeromq plugin
func run(plugin *node.Plugin) {

	plugin.LogInfo("Starting ZeroMQ Publisher (port " + strconv.Itoa(*PORT.Value) + ") ...")

	daemon.BackgroundWorker("ZeroMQ Publisher", func() {
		if err := startPublisher(plugin); err != nil {
			plugin.LogFailure("Stopping ZeroMQ Publisher: " + err.Error())
		} else {
			plugin.LogSuccess("Starting ZeroMQ Publisher (port " + strconv.Itoa(*PORT.Value) + ") ... done")
		}
	})
}

// Start the zmq publisher.
func startPublisher(plugin *node.Plugin) error {
	pub, err := NewPublisher()
	if err != nil {
		return err
	}
	publisher = pub

	return publisher.Start(*PORT.Value)
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
