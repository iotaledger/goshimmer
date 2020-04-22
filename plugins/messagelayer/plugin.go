package messagelayer

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messageparser"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagerequester"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
)

const (
	PluginName       = "MessageLayer"
	DBSequenceNumber = "seq"
)

var (
	PLUGIN           = node.NewPlugin(PluginName, node.Enabled, configure, run)
	MessageParser    *messageparser.MessageParser
	MessageRequester *messagerequester.MessageRequester
	TipSelector      *tipselector.TipSelector
	Tangle           *tangle.Tangle
	MessageFactory   *messagefactory.MessageFactory
	log              *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	// create instances
	MessageParser = messageparser.New()
	MessageRequester = messagerequester.New()
	TipSelector = tipselector.New()
	Tangle = tangle.New(database.GetBadgerInstance())

	// Setup MessageFactory (behavior + logging))
	MessageFactory = messagefactory.New(database.GetBadgerInstance(), local.GetInstance().LocalIdentity(), TipSelector, []byte(DBSequenceNumber))
	MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(Tangle.AttachMessage))
	MessageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("internal error in message factory: %v", err)
	}))

	// setup MessageParser
	MessageParser.Events.MessageParsed.Attach(events.NewClosure(func(msg *message.Message, peer *peer.Peer) {
		// TODO: ADD PEER
		Tangle.AttachMessage(msg)
	}))

	// setup MessageRequester
	Tangle.Events.MessageMissing.Attach(events.NewClosure(MessageRequester.ScheduleRequest))
	Tangle.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			MessageRequester.StopRequest(msg.Id())
		})
	}))

	// setup TipSelector
	Tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(TipSelector.AddTip)
	}))
}

func run(*node.Plugin) {
	_ = daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		MessageFactory.Shutdown()
		MessageParser.Shutdown()
		Tangle.Shutdown()
	}, shutdown.PriorityTangle)
}
