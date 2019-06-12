package instances

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc/instances/test"
	"github.com/iotaledger/goshimmer/packages/fpc"
)

var fpcInstance fpc.FPC

func Configure(plugin *node.Plugin) {
	gkp := func() ([]int) {
		return []
	}
	qn := func(txs []Hash, neighbor int) []Opinion {
		result := 0
		for _ := range txs {
			result.append(bool)
		}
		return result
	}

	fpcInstance = New(gkp, qn, parameters)
}

func Run(plugin *node.Plugin) {
}
