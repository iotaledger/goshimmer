package drng

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/consensus"
)

// PluginName is the name of the DRNG plugin.
const PluginName = "DRNG"

var (
	// Plugin is the plugin deps.DrngInstance of the DRNG plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	inbox     chan tangle.MessageID
	inboxSize = 100
)

type dependencies struct {
	dig.In
	Tangle       *tangle.Tangle
	DrngInstance *drng.DRNG
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
	inbox = make(chan tangle.MessageID, inboxSize)

	Plugin.Events.Init.Attach(events.NewClosure(func(_ *node.Plugin, container *dig.Container) {
		if err := container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("drng")); err != nil {
			Plugin.Panic(err)
		}

		if err := container.Provide(configureDRNG); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	configureEvents()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("dRNG-plugin", func(shutdownSignal <-chan struct{}) {
	loop:
		for {
			select {
			case <-shutdownSignal:
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
					parsedPayload, err := drng.PayloadFromMarshalUtil(marshalUtil)
					if err != nil {
						// TODO: handle error
						plugin.LogDebug(err)
						return
					}
					if err := deps.DrngInstance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
						// TODO: handle error
						plugin.LogDebug(err)
						return
					}
					plugin.LogDebug("New randomness: ", deps.DrngInstance.State[parsedPayload.InstanceID].Randomness())
				})
			}
		}

		Plugin.LogInfof("Stopping %s ... done", "dRNG-plugin")
	}, shutdown.PriorityFPC); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	// skip the event configuration if no committee has been configured.
	if len(deps.DrngInstance.State) == 0 {
		return
	}

	deps.Tangle.ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		select {
		case inbox <- messageID:
		default:
		}
	}))

	consensus.SetDRNGState(deps.DrngInstance.LoadState(consensus.FPCParameters.DRNGInstanceID))

	// Section to update the randomness for the dRNG ticker used by FPC.
	deps.DrngInstance.Events.Randomness.Attach(events.NewClosure(func(state *drng.State) {
		if state.Committee().InstanceID == consensus.FPCParameters.DRNGInstanceID {
			if ticker := consensus.DRNGTicker(); ticker != nil {
				ticker.UpdateRandomness(state.Randomness())
			}
		}
	}))
}
