package test

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/FPC/parameters"
	"github.com/iotaledger/goshimmer/plugins/fpc/network"
	"github.com/iotaledger/goshimmer/plugins/fpc/network/server"
	"github.com/iotaledger/goshimmer/plugins/fpc/prng/client"
)

var INSTANCE *fpc.Instance
var Events fpcEvents

func Configure(plugin *node.Plugin) {
	INSTANCE = fpc.New(network.GetKnownPeers, network.QueryNode, fpc.NewParameters())
	Events.NewFinalizedTxs = events.NewEvent(newFinalizedTxsCaller)
}

func Run(plugin *node.Plugin) {
	server.RunServer(plugin, INSTANCE)

	daemon.BackgroundWorker(func() {
		ticker := client.NewTicker()
		ticker.Connect(*parameters.PRNG_ADDRESS.Value + ":" + *parameters.PRNG_PORT.Value)
		INSTANCE.SubmitTxsForVoting(fpc.TxOpinion{"1", fpc.Like})
	ticker:
		for {
			select {
			case newRandom := <-ticker.C:
				INSTANCE.Tick(newRandom.Index, newRandom.Value)
				plugin.LogInfo(fmt.Sprintf("Round %v %v", newRandom.Index, INSTANCE.GetInterimOpinion("1")))
			case finalizedTxs := <-INSTANCE.FinalizedTxsChannel():
				// if len(finalizedTxs) == 0, an fpc round
				// ended with no new finalized transactions
				if len(finalizedTxs) > 0 {
					Events.NewFinalizedTxs.Trigger(finalizedTxs)
					plugin.LogInfo(fmt.Sprintf("Finalized txs %v", finalizedTxs))
				}
			case <-daemon.ShutdownSignal:
				plugin.LogInfo("Stopping FPC Processor ...")
				break ticker
			}
		}
		plugin.LogSuccess("Stopping FPC Processor ... done")
	})

	// TODO: REMOVE THIS
	// Example of how to use the event
	daemon.BackgroundWorker(func() {
		Events.NewFinalizedTxs.Attach(events.NewClosure(func(txs []fpc.TxOpinion) {
			plugin.LogInfo(fmt.Sprintf("EVENT TEST: %v", txs))
		}))
	})
}
