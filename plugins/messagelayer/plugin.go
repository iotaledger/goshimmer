package messagelayer

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName defines the plugin name.
	PluginName = "MessageLayer"
)

var (
	// plugin is the plugin instance of the message layer plugin.
	plugin           *node.Plugin
	pluginOnce       sync.Once
	messageParser    *tangle.MessageParser
	msgParserOnce    sync.Once
	messageRequester *tangle.MessageRequester
	msgReqOnce       sync.Once
	tipSelector      *tangle.MessageTipSelector
	tipSelectorOnce  sync.Once
	_tangle          *tangle.Tangle
	tangleOnce       sync.Once
	messageFactory   *tangle.MessageFactory
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
func MessageParser() *tangle.MessageParser {
	msgParserOnce.Do(func() {
		messageParser = tangle.NewMessageParser()
	})
	return messageParser
}

// TipSelector gets the tipSelector instance.
func TipSelector() *tangle.MessageTipSelector {
	tipSelectorOnce.Do(func() {
		tipSelector = tangle.NewMessageTipSelector()
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
func MessageFactory() *tangle.MessageFactory {
	msgFactoryOnce.Do(func() {
		messageFactory = tangle.NewMessageFactory(database.Store(), []byte(tangle.DBSequenceNumber), local.GetInstance().LocalIdentity(), TipSelector())
	})
	return messageFactory
}

// MessageRequester gets the messageRequester instance.
func MessageRequester() *tangle.MessageRequester {
	msgReqOnce.Do(func() {
		// load all missing messages on start up
		messageRequester = tangle.NewMessageRequester(Tangle().MissingMessages())
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

	// setup solidification
	// TODO: replace solidification with the timestamp check
	_tangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		_tangle.SolidifyMessage(cachedMsgEvent.Message, cachedMsgEvent.MessageMetadata)
	}))

	// Setup messageFactory (behavior + logging))
	messageFactory = MessageFactory()
	messageFactory.Events.MessageConstructed.Attach(events.NewClosure(_tangle.StoreMessage))
	messageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("internal error in message factory: %v", err)
	}))

	// setup messageParser
	messageParser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *tangle.MessageParsedEvent) {
		// TODO: ADD PEER
		_tangle.StoreMessage(msgParsedEvent.Message)
	}))

	// setup messageRequester
	_tangle.MessageStore.Events.MessageMissing.Attach(events.NewClosure(messageRequester.StartRequest))
	_tangle.MessageStore.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()
		cachedMsgEvent.Message.Consume(func(msg *tangle.Message) {
			messageRequester.StopRequest(msg.ID())
		})
	}))

	// setup tipSelector
	_tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()
		cachedMsgEvent.Message.Consume(tipSelector.AddTip)
	}))

	MessageRequester().Events.MissingMessageAppeared.Attach(events.NewClosure(func(missingMessageAppeared *tangle.MissingMessageAppearedEvent) {
		_tangle.DeleteMissingMessage(missingMessageAppeared.ID)
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
