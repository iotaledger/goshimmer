package networkdelay

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	messageTangle "github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "NetworkDelay"
)

var (
	// App is the "plugin" instance of the network delay application.
	App = node.NewPlugin(PluginName, node.Enabled, configure, run)

	// log holds a reference to the logger used by this app.
	log *logger.Logger
)

func configure(_ *node.Plugin) {
	// configure logger
	log = logger.NewLogger(PluginName)

	configureWebAPI()

	// subscribe to message-layer
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))
}

func run(_ *node.Plugin) {

}

func onReceiveMessageFromMessageLayer(cachedMessage *message.CachedMessage, cachedMessageMetadata *messageTangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	solidMessage := cachedMessage.Unwrap()
	if solidMessage == nil {
		log.Debug("failed to unpack solid message from message layer")

		return
	}

	messagePayload := solidMessage.Payload()
	if messagePayload.Type() != Type {
		return
	}

	networkDelayObject, ok := messagePayload.(*Object)
	if !ok {
		log.Info("could not cast payload to network delay object")

		return
	}

	log.Info("Received: ", networkDelayObject)
	now := time.Now().UnixNano()
	log.Infof("local time: %d, delta: %d", now, now-networkDelayObject.sentTime)
}
