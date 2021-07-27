package drng

import (
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// PluginName is the name of the DRNG plugin.
const PluginName = "DRNG"

var (
	// plugin is the plugin instance of the DRNG plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once
	instance   *drng.DRNG
	once       sync.Once

	inbox     chan tangle.MessageID
	inboxSize = 100
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
		inbox = make(chan tangle.MessageID, inboxSize)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("dRNG-plugin", func(shutdownSignal <-chan struct{}) {
	loop:
		for {
			select {
			case <-shutdownSignal:
				plugin.LogInfof("Stopping %s ...", "dRNG-plugin")
				break loop
			case messageID := <-inbox:
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
						// TODO: handle error
						plugin.LogDebug(err)
						return
					}
					if err := instance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
						// TODO: handle error
						plugin.LogDebug(err)
						return
					}
					plugin.LogDebug("New randomness: ", instance.State[parsedPayload.InstanceID].Randomness())
				})
			}
		}

		plugin.LogInfof("Stopping %s ... done", "dRNG-plugin")
	}, shutdown.PriorityFPC); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	// skip the event configuration if no committee has been configured.
	if len(Instance().State) == 0 {
		return
	}

	messagelayer.Tangle().Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		select {
		case inbox <- messageID:
		default:
		}
	}))
}
