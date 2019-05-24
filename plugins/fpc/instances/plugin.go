package instances

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc/instances/chosenquorum"
)

func Configure(plugin *node.Plugin) {
	chosenquorum.Configure(plugin)
}

func Run(plugin *node.Plugin) {
	chosenquorum.Run(plugin)
}
