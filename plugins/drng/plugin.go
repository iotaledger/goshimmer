package drng

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the DRNG plugin.
const PluginName = "DRNG"

var (
	// plugin is the plugin instance of the DRNG plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once
	instance   *drng.DRNG
	once       sync.Once
	log        *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	configureEvents()
}

func run(*node.Plugin) {}

func configureEvents() {
	// skip the event configuration if no committee has been configured.
	if len(Instance().State) == 0 {
		return
	}

	messagelayer.Tangle().Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			if msg.Payload().Type() != drng.PayloadType {
				return
			}
			if len(msg.Payload().Bytes()) < drng.HeaderLength {
				return
			}
			marshalUtil := marshalutil.New(msg.Payload().Bytes())
			parsedPayload, err := drng.PayloadFromMarshalUtil(marshalUtil)
			if err != nil {
				//TODO: handle error
				log.Debug(err)
				return
			}
			if err := instance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
				//TODO: handle error
				log.Debug(err)
				return
			}
			log.Debug("New randomness: ", instance.State[parsedPayload.InstanceID].Randomness())
		})
	}))
}
