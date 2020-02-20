package tangle

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/tipselector"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/transactionparser"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/transactionrequester"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Tangle", node.Enabled, configure, run)

var TransactionParser *transactionparser.TransactionParser

var TransactionRequester *transactionrequester.TransactionRequester

var TipSelector *tipselector.TipSelector

var Instance *tangle.Tangle

var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger("Tangle")

	// create instances
	TransactionParser = transactionparser.New()
	TransactionRequester = transactionrequester.New()
	TipSelector = tipselector.New()
	Instance = tangle.New(database.GetBadgerInstance(), storageprefix.MainNet)

	// setup TransactionParser
	TransactionParser.Events.TransactionParsed.Attach(events.NewClosure(func(transaction *transaction.Transaction, peer *peer.Peer) {
		// TODO: ADD PEER

		Instance.AttachTransaction(transaction)
	}))

	// setup TransactionRequester
	Instance.Events.TransactionMissing.Attach(events.NewClosure(TransactionRequester.ScheduleRequest))
	Instance.Events.MissingTransactionReceived.Attach(events.NewClosure(func(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *transactionmetadata.CachedTransactionMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			TransactionRequester.StopRequest(transaction.GetId())
		})
	}))

	// setup TipSelector
	Instance.Events.TransactionSolid.Attach(events.NewClosure(func(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *transactionmetadata.CachedTransactionMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(TipSelector.AddTip)
	}))
}

func run(*node.Plugin) {
	_ = daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		TransactionParser.Shutdown()
		Instance.Shutdown()
	}, shutdown.ShutdownPriorityTangle)
}
