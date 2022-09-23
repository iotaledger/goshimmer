package blockissuer

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/blockfactory"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter"
	"github.com/iotaledger/goshimmer/packages/protocol"
)

// PluginName is the name of the spammer plugin.
const PluginName = "BlockIssuer"

var (
	// Plugin is the plugin instance of the spammer plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

type dependencies struct {
	dig.In

	Local    *peer.Local
	Protocol *protocol.Protocol
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(createBlockIssuer); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func createBlockIssuer() *blockissuer.BlockIssuer {
	//TODO: retainer needs to use protocol instead of engine
	// TODO: retainer db needs to be be pruned (create configuration)
	return blockissuer.New(deps.Protocol, deps.Local.LocalIdentity(), blockissuer.WithBlockFactoryOptions(
		[]options.Option[blockfactory.Factory]{
			blockfactory.WithTipSelectionRetryInterval(Parameters.BlockFactory.TipSelectionRetryInterval),
			blockfactory.WithTipSelectionTimeout(Parameters.BlockFactory.TipSelectionTimeout),
		},
	), blockissuer.WithRateSetterOptions([]options.Option[ratesetter.RateSetter]{
		ratesetter.WithPause(Parameters.RateSetter.Pause),
		ratesetter.WithInitialRate(Parameters.RateSetter.Initial),
		ratesetter.WithEnabled(Parameters.RateSetter.Enable),
	}), blockissuer.WithIgnoreBootstrappedFlag(Parameters.IgnoreBootstrappedFlag),
	)
}
