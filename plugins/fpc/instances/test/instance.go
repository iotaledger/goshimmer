package test

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"

	"time"
)

var INSTANCE *fpc.FPC

func Configure(plugin *node.Plugin) {
	getKnownPeers := func() []int {
		return []int{1, 2, 3, 4, 5}
	}

	queryNode := func(txs []fpc.Hash, node int) []fpc.Opinion {
		output := make([]fpc.Opinion, len(txs))
		for tx := range txs {
			output[tx] = fpc.Like
		}
		return output
	}

	INSTANCE = fpc.New(getKnownPeers, queryNode, fpc.NewParameters())

	// INSTANCE.VoteOnTxs()
	// INSTANCE.GetInterimOpinion()

}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker(func() {
		ticker := time.NewTicker(5000 * time.Millisecond)
		round := 0
		INSTANCE.VoteOnTxs(fpc.TxOpinion{1, fpc.Like})
		for {
			select {
			case <-ticker.C:
				round++
				INSTANCE.Tick(uint64(round), 0.7)
				plugin.LogInfo(fmt.Sprintf("Round %v %v", round, INSTANCE.Debug_GetOpinionHistory()))
			case finalizedTxs := <-INSTANCE.FinalizedTxs:
				if len(finalizedTxs) > 0 {
					plugin.LogInfo(fmt.Sprintf("Finalized txs %v %v", finalizedTxs, INSTANCE.Debug_GetOpinionHistory()))
				}
			}
		}
	})
}
