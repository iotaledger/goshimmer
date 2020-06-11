package testsnapshots

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName is the plugin name of the bootstrap plugin.
	PluginName = "TestSnapshots"
)

var (
	// Plugin is the plugin instance of the bootstrap plugin.
	Plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	log    *logger.Logger

	// addresses for snapshots
	address0, _ = address.FromBase58("JaMauTaTSVBNc13edCCvBK9fZxZ1KKW5fXegT1B7N9jY")
)

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	valuetransfers.Tangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			address0: []*balance.Balance{
				balance.New(balance.ColorIOTA, 10000000),
			},
		},
	})

	log.Infof("load snapshots to tangle")
}

func run(_ *node.Plugin) {}
