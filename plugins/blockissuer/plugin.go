package blockissuer

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/blockfactory"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter"
	"github.com/iotaledger/goshimmer/packages/protocol"
	protocolParams "github.com/iotaledger/goshimmer/plugins/protocol"
)

// PluginName is the name of the spammer plugin.
const PluginName = "BlockIssuer"

var (
	// Plugin is the plugin instance of the spammer plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	BlockIssuer *blockissuer.BlockIssuer
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
	deps.BlockIssuer.Events.Error.Attach(event.NewClosure(func(err error) {
		Plugin.LogErrorf("Error in BlockIssuer: %s", err)
	}))
}

func createBlockIssuer(local *peer.Local, protocol *protocol.Protocol) *blockissuer.BlockIssuer {

	rateSetterMode := ratesetter.ParseRateSetterMode(Parameters.RateSetter.Mode)
	var rateSetter ratesetter.RateSetter

	switch rateSetterMode {
	case ratesetter.AIMDMode:
		rateSetter = ratesetter.NewAIMD(protocol, local.LocalIdentity().ID(),
			ratesetter.WithPause(Parameters.RateSetter.Pause),
			ratesetter.WithInitialRate(Parameters.RateSetter.Initial),
			ratesetter.WithSchedulerRateAIMD(protocolParams.SchedulerParameters.Rate),
		)
	case ratesetter.DeficitMode:
		rateSetter = ratesetter.NewDeficit(protocol, local.LocalIdentity().ID(),
			ratesetter.WithSchedulerRateDeficit(protocolParams.SchedulerParameters.Rate),
		)
	default:
		rateSetter = ratesetter.NewDisabled()
	}

	return blockissuer.New(protocol, local.LocalIdentity(),
		blockissuer.WithBlockFactoryOptions(
			blockfactory.WithTipSelectionRetryInterval(Parameters.BlockFactory.TipSelectionRetryInterval),
			blockfactory.WithTipSelectionTimeout(Parameters.BlockFactory.TipSelectionTimeout),
		),
		blockissuer.WithRateSetterMode(rateSetterMode),
		blockissuer.WithRateSetter(rateSetter),
		blockissuer.WithIgnoreBootstrappedFlag(Parameters.IgnoreBootstrappedFlag),
	)
}
