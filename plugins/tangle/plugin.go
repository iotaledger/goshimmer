package tangle

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/transactionparser"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/transactionrequester"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
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

var Instance *messagelayer.Tangle

var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger("Tangle")

	// create instances
	TransactionParser = transactionparser.New()
	TransactionRequester = transactionrequester.New()
	TipSelector = tipselector.New()
	Instance = messagelayer.New(database.GetBadgerInstance(), storageprefix.MainNet)

	// setup TransactionParser
	TransactionParser.Events.TransactionParsed.Attach(events.NewClosure(func(transaction *message.Message, peer *peer.Peer) {
		// TODO: ADD PEER

		Instance.AttachTransaction(transaction)
	}))

	// setup TransactionRequester
	Instance.Events.TransactionMissing.Attach(events.NewClosure(TransactionRequester.ScheduleRequest))
	Instance.Events.MissingTransactionReceived.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *transactionmetadata.CachedMessageMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *message.Message) {
			TransactionRequester.StopRequest(transaction.GetId())
		})
	}))

	// setup TipSelector
	Instance.Events.TransactionSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *transactionmetadata.CachedMessageMetadata) {
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
