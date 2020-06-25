package messagelayer

import (
	"sync"

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
	plugin           *node.Plugin
	pluginOnce       sync.Once
	messageParser    *messageparser.MessageParser
	msgParserOnce    sync.Once
	messageRequester *messagerequester.MessageRequester
	msgReqOnce       sync.Once
	tipSelector      *tipselector.TipSelector
	tipSelectorOnce  sync.Once
	_tangle          *tangle.Tangle
	tangleOnce       sync.Once
	messageFactory   *messagefactory.MessageFactory
	msgFactoryOnce   sync.Once
	log              *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// MessageParser gets the messageParser instance.
func MessageParser() *messageparser.MessageParser {
	msgParserOnce.Do(func() {
		messageParser = messageparser.New()
	})
	return messageParser
}

// TipSelector gets the tipSelector instance.
func TipSelector() *tipselector.TipSelector {
	tipSelectorOnce.Do(func() {
		tipSelector = tipselector.New()
	})
	return tipSelector
}

// Tangle gets the tangle instance.
func Tangle() *tangle.Tangle {
	tangleOnce.Do(func() {
		store := database.Store()
		_tangle = tangle.New(store)
	})
	return _tangle
}

// MessageFactory gets the messageFactory instance.
func MessageFactory() *messagefactory.MessageFactory {
	msgFactoryOnce.Do(func() {
		messageFactory = messagefactory.New(database.Store(), []byte(DBSequenceNumber), local.GetInstance().LocalIdentity(), TipSelector())
	})
	return messageFactory
}

// MessageRequester gets the messageRequester instance.
func MessageRequester() *messagerequester.MessageRequester {
	msgReqOnce.Do(func() {
		messageRequester = messagerequester.New()
	})
	return messageRequester
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	// create instances
	messageParser = MessageParser()
	messageRequester = MessageRequester()
	tipSelector = TipSelector()
	_tangle = Tangle()

	// Setup messageFactory (behavior + logging))
	messageFactory = MessageFactory()
	messageFactory.Events.MessageConstructed.Attach(events.NewClosure(_tangle.AttachMessage))
	messageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("internal error in message factory: %v", err)
	}))

	// setup messageParser
	messageParser.Events.MessageParsed.Attach(events.NewClosure(func(msg *message.Message, peer *peer.Peer) {
		// TODO: ADD PEER
		_tangle.AttachMessage(msg)
	}))

	// setup messageRequester
	_tangle.Events.MessageMissing.Attach(events.NewClosure(messageRequester.ScheduleRequest))
	_tangle.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			messageRequester.StopRequest(msg.Id())
		})
	}))

	// setup tipSelector
	_tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(tipSelector.AddTip)
	}))
}

func run(*node.Plugin) {

	if err := daemon.BackgroundWorker("Tangle[MissingMessagesMonitor]", func(shutdownSignal <-chan struct{}) {
		_tangle.MonitorMissingMessages(shutdownSignal)
	}, shutdown.PriorityMissingMessagesMonitoring); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		messageFactory.Shutdown()
		messageParser.Shutdown()
		_tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

}
