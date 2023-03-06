package manainitializer

import (
	// import required to profile.
	_ "net/http/pprof"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/lo"
)

// PluginName is the name of the profiling plugin.
const PluginName = "ManaInitializer"

var (
	// Plugin is the profiling plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Local *peer.Local
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
}

func run(_ *node.Plugin) {
	api := client.NewGoShimmerAPI(Parameters.FaucetAPI)
	pledgeAddress := Parameters.Address
	if pledgeAddress == "" {
		pledgeAddress = seed.NewSeed(lo.PanicOnErr(deps.Local.PublicKey().Bytes())).Address(0).Base58()
	}
	res, err := api.SendFaucetRequestAPI(pledgeAddress, -1, deps.Local.ID().EncodeBase58(), deps.Local.ID().EncodeBase58())
	if err != nil {
		Plugin.LogWarnf("Could not fulfill faucet request: %v", err)
		return
	}

	if !res.Success {
		Plugin.LogWarnf("Could not fulfill faucet request: %v", res.Error)
		return
	}
	Plugin.LogInfof("Successfully requested initial mana!")
}
