package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"
)

const name = "DRNG" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var (
	Instance *drng.Instance
	log      *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)
	Instance = drng.New()
	configureEvents()
}

func run(*node.Plugin) {}

func configureEvents() {
	messagelayer.Tangle.Events.TransactionSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()

		cachedMessage.Consume(func(msg *message.Message) {
			marshalUtil := marshalutil.New(msg.Payload().Bytes())
			parsedPayload, err := payload.Parse(marshalUtil)
			if err != nil {
				//TODO: handle error
				log.Info(err)
				return
			}
			if err := Instance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
				//TODO: handle error
				log.Info(err)
				return
			}
			log.Info(Instance.State.Randomness())
		})
	}))

}
