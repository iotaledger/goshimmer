package drng

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the DRNG plugin.
const PluginName = "DRNG"

var (
	// Plugin is the plugin instance of the DRNG plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	inbox     chan tangle.MessageID
	inboxSize = 100
)

type dependencies struct {
	dig.In
	Tangle       *tangle.Tangle
	DRNGInstance *drng.DRNG
	DRNGTTicker  *drng.Ticker `optional:"true"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
	inbox = make(chan tangle.MessageID, inboxSize)

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
		if err := event.Container.Provide(configureDRNG); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	configureEvents()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("dRNG-plugin", func(ctx context.Context) {
	loop:
		for {
			select {
			case <-ctx.Done():
				plugin.LogInfof("Stopping %s ...", "dRNG-plugin")
				break loop
			case messageID := <-inbox:
				deps.Tangle.Storage.Message(messageID).Consume(func(msg *tangle.Message) {
					if msg.Payload().Type() != drng.PayloadType {
						return
					}
					if len(msg.Payload().Bytes()) < drng.HeaderLength {
						return
					}
					marshalUtil := marshalutil.New(msg.Payload().Bytes())
					parsedPayload, err := drng.CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
					if err != nil {
						// TODO: handle error
						plugin.LogDebug(err)
						return
					}
					if err = deps.DRNGInstance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
						// TODO: handle error
						plugin.LogDebug(err)
						return
					}
					plugin.LogDebug("New randomness: ", deps.DRNGInstance.State[parsedPayload.InstanceID].Randomness())
				})
			}
		}

		Plugin.LogInfof("Stopping %s ... done", "dRNG-plugin")
	}, shutdown.PriorityDRNG); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	// skip the event configuration if no committee has been configured.
	if len(deps.DRNGInstance.State) == 0 {
		return
	}

	deps.Tangle.ApprovalWeightManager.Events.MessageProcessed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		select {
		case inbox <- messageID:
		default:
		}
	}))
}
