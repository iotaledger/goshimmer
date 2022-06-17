package manainitializer

import (
	"fmt"
	// import required to profile
	_ "net/http/pprof"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
)

// PluginName is the name of the profiling plugin.
const PluginName = "ManaInitializer"

var (
	// Plugin is the profiling plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	log *logger.Logger
)

type dependencies struct {
	dig.In

	Local *peer.Local
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	api := client.NewGoShimmerAPI(Parameters.FaucetAPI)
	pledgeAddress := Parameters.Address
	if pledgeAddress == "" {
		fmt.Printf("deps %+v\n", deps)
		pledgeAddress = seed.NewSeed(deps.Local.PublicKey().Bytes()).Address(0).Base58()
	}
	res, err := api.SendFaucetRequestAPI(pledgeAddress, -1, deps.Local.ID().EncodeBase58(), deps.Local.ID().EncodeBase58())
	if err != nil {
		log.Warnf("Could not fulfill faucet request: %v", err)
		return
	}

	if !res.Success {
		log.Warnf("Could not fulfill faucet request: %v", res.Error)
		return
	}
	log.Infof("Successfully requested initial mana!")
}
