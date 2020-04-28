package drng

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/drng"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the DRNG plugin.
const PluginName = "DRNG"

var (
	// Plugin is the plugin instance of the DRNG plugin.
	Plugin   = node.NewPlugin(PluginName, node.Enabled, configure, run)
	instance *drng.DRNG
	once     sync.Once
	log      *logger.Logger
)

func configure(_ *node.Plugin) {
	configureEvents()
}

func run(*node.Plugin) {}

func configureEvents() {
	instance := Instance()
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()

		cachedMessage.Consume(func(msg *message.Message) {
			if msg.Payload().Type() != payload.Type {
				return
			}
			if len(msg.Payload().Bytes()) < header.Length {
				return
			}
			marshalUtil := marshalutil.New(msg.Payload().Bytes())
			parsedPayload, err := payload.Parse(marshalUtil)
			if err != nil {
				//TODO: handle error
				log.Info(err)
				return
			}
			if err := instance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
				//TODO: handle error
				log.Info(err)
				return
			}
			log.Info(instance.State.Randomness())
		})
	}))
}
