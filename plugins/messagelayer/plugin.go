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
	// PluginName defines the plugin name.
	PluginName = "MessageLayer"
	// DBSequenceNumber defines the db sequence number.
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
		// load all missing messages on start up
		messageRequester = messagerequester.New(messageExists, Tangle().MissingMessages())
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
	_tangle.Events.MessageMissing.Attach(events.NewClosure(messageRequester.StartRequest))
	_tangle.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			messageRequester.StopRequest(msg.ID())
		})
	}))

	// setup tipSelector
	_tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(tipSelector.AddTip)
	}))

	MessageRequester().Events.MissingMessageAppeared.Attach(events.NewClosure(func(id message.ID) {
		_tangle.DeleteMissingMessage(id)
	}))
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		messageFactory.Shutdown()
		_tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// messageExists tells if a given message is present in the node
func messageExists(msgID message.ID) bool {
	cachedMessage := Tangle().Message(msgID)
	defer cachedMessage.Release()
	return cachedMessage.Exists()
}
