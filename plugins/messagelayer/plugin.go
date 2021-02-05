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
	tangleInstance   *tangle.Tangle
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
		tangleInstance = tangle.New(store)
	})
	return tangleInstance
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
		messageRequester = tangle.NewMessageRequester(Tangle().Storage.MissingMessages())
	})
	return messageRequester
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	// create instances
	messageParser = MessageParser()
	messageRequester = MessageRequester()
	tipSelector = TipSelector()
	tangleInstance = Tangle()

	// Setup messageFactory (behavior + logging))
	messageFactory = MessageFactory()
	messageFactory.Events.MessageConstructed.Attach(events.NewClosure(tangleInstance.Storage.StoreMessage))
	messageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("internal error in message factory: %v", err)
	}))

	// setup messageParser
	messageParser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *tangle.MessageParsedEvent) {
		// TODO: ADD PEER
		tangleInstance.Storage.StoreMessage(msgParsedEvent.Message)
	}))

	// setup messageRequester
	tangleInstance.Storage.Events.MessageMissing.Attach(events.NewClosure(messageRequester.StartRequest))
	tangleInstance.Storage.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()
		cachedMsgEvent.Message.Consume(func(msg *tangle.Message) {
			messageRequester.StopRequest(msg.ID())
		})
	}))

	// setup tipSelector
	tangleInstance.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		Tangle().Storage.Message(messageID).Consume(tipSelector.AddTip)
	}))

	MessageRequester().Events.MissingMessageAppeared.Attach(events.NewClosure(func(missingMessageAppeared *tangle.MissingMessageAppearedEvent) {
		tangleInstance.Storage.DeleteMissingMessage(missingMessageAppeared.ID)
	}))
}

func run(*node.Plugin) {
	messageParser.Setup()
	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		messageFactory.Shutdown()
		tangleInstance.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
