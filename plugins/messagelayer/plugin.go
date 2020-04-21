package messagelayer

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/transactionparser"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/transactionrequester"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
)

const (
	PLUGIN_NAME        = "MessageLayer"
	DB_SEQUENCE_NUMBER = "seq"
)

var PLUGIN = node.NewPlugin(PLUGIN_NAME, node.Enabled, configure, run)

var TransactionParser *transactionparser.TransactionParser

var TransactionRequester *transactionrequester.TransactionRequester

var TipSelector *tipselector.TipSelector

var Tangle *tangle.Tangle

var MessageFactory *messagefactory.MessageFactory

var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)

	// create instances
	TransactionParser = transactionparser.New()
	TransactionRequester = transactionrequester.New()
	TipSelector = tipselector.New()
	Tangle = tangle.New(database.GetBadgerInstance())

	// Setup MessageFactory (behavior + logging))
	MessageFactory = messagefactory.New(database.GetBadgerInstance(), local.GetInstance().LocalIdentity(), TipSelector, []byte(DB_SEQUENCE_NUMBER))
	MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(Tangle.AttachMessage))
	MessageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("Error in MessageFactory: %v", err)
	}))

	// setup TransactionParser
	TransactionParser.Events.TransactionParsed.Attach(events.NewClosure(func(transaction *message.Message, peer *peer.Peer) {
		// TODO: ADD PEER

		Tangle.AttachMessage(transaction)
	}))

	// setup TransactionRequester
	Tangle.Events.TransactionMissing.Attach(events.NewClosure(TransactionRequester.ScheduleRequest))
	Tangle.Events.MissingTransactionReceived.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *tangle.CachedMessageMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *message.Message) {
			TransactionRequester.StopRequest(transaction.Id())
		})
	}))

	// setup TipSelector
	Tangle.Events.TransactionSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *tangle.CachedMessageMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(TipSelector.AddTip)
	}))
}

func run(*node.Plugin) {
	_ = daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		MessageFactory.Shutdown()
		TransactionParser.Shutdown()
		Tangle.Shutdown()
	}, shutdown.PriorityTangle)
}
