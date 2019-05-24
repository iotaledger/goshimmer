package instances

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc/instances/test"
)

func Configure(plugin *node.Plugin) {
	//chosenquorum.Configure(plugin)
	test.Configure(plugin)
}

func Run(plugin *node.Plugin) {
	//chosenquorum.Run(plugin)
	test.Run(plugin)
}
