package node

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Node"

var (
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	P2PManager *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func provide() (result providerResult) {
	result.Network = network.New(deps.P2PManager, func(id models.BlockID) *models.Block {
		fmt.Println(result.Protocol)

		return nil
	}, Plugin.Logger())

	result.Protocol = protocol.New(result.Network)

	return
}

type providerResult struct {
	Protocol *protocol.Protocol
	Network  *network.Network

	dig.Out
}
