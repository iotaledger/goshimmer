package fpc

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc/network"
	"github.com/iotaledger/goshimmer/plugins/fpc/network/server"
	"github.com/iotaledger/goshimmer/plugins/fpc/parameters"
	"github.com/iotaledger/goshimmer/plugins/fpc/prng/client"
)

// INSTANCE points to the fpc instance (concrete type)
var INSTANCE *fpc.Instance

// Events exposes fpc events
var Events fpcEvents

func configureFPC(plugin *node.Plugin) {
	INSTANCE = fpc.New(network.GetKnownPeers, network.QueryNode, fpc.NewParameters())
	Events.VotingDone = events.NewEvent(votingDoneCaller)
}

func runFPC(plugin *node.Plugin) {
	server.RunServer(plugin, INSTANCE)

	daemon.BackgroundWorker("FPC Random Number Listener", func() {
		ticker := client.NewTicker()
		ticker.Connect(*parameters.PRNG_ADDRESS.Value + ":" + *parameters.PRNG_PORT.Value)
	ticker:
		for {
			select {
			case newRandom := <-ticker.C:
				INSTANCE.Tick(newRandom.Index, newRandom.Value)
				plugin.LogInfo(fmt.Sprintf("Round %v", newRandom.Index))
			case finalizedTxs := <-INSTANCE.FinalizedTxsChannel():
				// if len(finalizedTxs) == 0, an fpc round
				// ended with no new finalized transactions
				if len(finalizedTxs) > 0 {
					Events.VotingDone.Trigger(finalizedTxs)
				}
			case <-daemon.ShutdownSignal:
				plugin.LogInfo("Stopping FPC Processor ...")
				break ticker
			}
		}
		plugin.LogSuccess("Stopping FPC Processor ... done")
	})
}
