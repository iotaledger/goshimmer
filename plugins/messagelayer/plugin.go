package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messageparser"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagerequester"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	PluginName       = "MessageLayer"
	DBSequenceNumber = "seq"
)

var (
	// plugin is the plugin instance of the message layer plugin.
	plugin           = node.NewPlugin(PluginName, node.Enabled, configure, run)
	messageParser    *messageparser.MessageParser
	messageRequester *messagerequester.MessageRequester
	tipSelector      *tipselector.TipSelector
	tngle            *tangle.Tangle
	messageFactory   *messagefactory.MessageFactory
	log              *logger.Logger
)

// Gets the plugin instance
func Plugin() *node.Plugin {
	return plugin
}

// Gets the messageParser instance
func MessageParser() *messageparser.MessageParser {
	return messageParser
}

// Gets the tipSelector instance
func TipSelector() *tipselector.TipSelector {
	return tipSelector
}

// Gets the tangle instance
func Tangle() *tangle.Tangle {
	return tngle
}

// Gets the messageFactory instance
func MessageFactory() *messagefactory.MessageFactory {
	return messageFactory
}

// Gets the messageRequester instance
func MessageRequester() *messagerequester.MessageRequester {
	return messageRequester
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	store := database.Store()

	// create instances
	messageParser = messageparser.New()
	messageRequester = messagerequester.New()
	tipSelector = tipselector.New()
	tngle = tangle.New(store)

	// Setup messageFactory (behavior + logging))
	messageFactory = messagefactory.New(database.Store(), local.GetInstance().LocalIdentity(), tipSelector, []byte(DBSequenceNumber))
	messageFactory.Events.MessageConstructed.Attach(events.NewClosure(tngle.AttachMessage))
	messageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("internal error in message factory: %v", err)
	}))

	// setup messageParser
	messageParser.Events.MessageParsed.Attach(events.NewClosure(func(msg *message.Message, peer *peer.Peer) {
		// TODO: ADD PEER
		tngle.AttachMessage(msg)
	}))

	// setup messageRequester
	tngle.Events.MessageMissing.Attach(events.NewClosure(messageRequester.ScheduleRequest))
	tngle.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			messageRequester.StopRequest(msg.Id())
		})
	}))

	// setup tipSelector
	tngle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(tipSelector.AddTip)
	}))
}

func run(*node.Plugin) {

	if err := daemon.BackgroundWorker("tngle[MissingMessagesMonitor]", func(shutdownSignal <-chan struct{}) {
		tngle.MonitorMissingMessages(shutdownSignal)
	}, shutdown.PriorityMissingMessagesMonitoring); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	if err := daemon.BackgroundWorker("tngle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		messageFactory.Shutdown()
		messageParser.Shutdown()
		tngle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

}
