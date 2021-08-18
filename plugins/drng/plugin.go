package drng

import (
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// PluginName is the name of the DRNG plugin.
const PluginName = "DRNG"

var (
	// Plugin is the plugin instance of the DRNG plugin.
	Plugin   *node.Plugin
	deps     dependencies
	instance *drng.DRNG
	once     sync.Once

	inbox     chan tangle.MessageID
	inboxSize = 100
)

type dependencies struct {
	dig.In
	Tangle       *tangle.Tangle
	DrngInstance *drng.DRNG
}

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	inbox = make(chan tangle.MessageID, inboxSize)

	Plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := dependencyinjection.Container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("drng")); err != nil {
			panic(err)
		}
		if err := dependencyinjection.Container.Provide(func() *drng.DRNG {
			drngInstance := configureDRNG()
			return drngInstance
		}); err != nil {
			panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}
	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("dRNG-plugin", func(shutdownSignal <-chan struct{}) {
	loop:
		for {
			select {
			case <-shutdownSignal:
				Plugin.LogInfof("Stopping %s ...", "dRNG-plugin")
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
						Plugin.LogDebug(err)
						return
					}
					if err := instance.Dispatch(msg.IssuerPublicKey(), msg.IssuingTime(), parsedPayload); err != nil {
						// TODO: handle error
						Plugin.LogDebug(err)
						return
					}
					Plugin.LogDebug("New randomness: ", instance.State[parsedPayload.InstanceID].Randomness())
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

	messagelayer.SetDRNGState(deps.DrngInstance.LoadState(messagelayer.FPCParameters.DRNGInstanceID))

	// Section to update the randomness for the dRNG ticker used by FPC.
	deps.DrngInstance.Events.Randomness.Attach(events.NewClosure(func(state *drng.State) {
		if state.Committee().InstanceID == messagelayer.FPCParameters.DRNGInstanceID {
			if ticker := messagelayer.DRNGTicker(); ticker != nil {
				ticker.UpdateRandomness(state.Randomness())
			}
		}
	}))
}
