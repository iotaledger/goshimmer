package test

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/FPC/parameters"
	"github.com/iotaledger/goshimmer/plugins/fpc/prng/client"
)

var INSTANCE *fpc.Instance

func Configure(plugin *node.Plugin) {
	getKnownPeers := func() []int {
		return []int{1, 2, 3, 4, 5}
	}

	queryNode := func(txs []fpc.ID, node int) []fpc.Opinion {
		output := make([]fpc.Opinion, len(txs))
		for tx := range txs {
			output[tx] = fpc.Like
		}
		return output
	}

	INSTANCE = fpc.New(getKnownPeers, queryNode, fpc.NewParameters())
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker(func() {
		ticker := client.NewTicker()
		ticker.Connect(*parameters.PRNG_ADDRESS.Value + ":" + *parameters.PRNG_PORT.Value)
		INSTANCE.SubmitTxsForVoting(fpc.TxOpinion{"1", fpc.Like})
		for {
			select {
			case newRandom := <-ticker.C:
				INSTANCE.Tick(newRandom.Index, newRandom.Value)
				plugin.LogInfo(fmt.Sprintf("Round %v %v", newRandom.Index, INSTANCE.GetInterimOpinion("1")))
			case finalizedTxs := <-INSTANCE.FinalizedTxsChannel():
				if len(finalizedTxs) > 0 {
					plugin.LogInfo(fmt.Sprintf("Finalized txs %v", finalizedTxs))
				}
			}
		}
	})
}
