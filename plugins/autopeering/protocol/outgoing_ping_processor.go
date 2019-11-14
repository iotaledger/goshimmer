package protocol

import (
	"math/rand"
	"time"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
	"github.com/iotaledger/hive.go/events"
)

var lastPing time.Time

func createOutgoingPingProcessor(plugin *node.Plugin) func() {
	return func() {
		plugin.LogInfo("Starting Ping Processor ...")
		plugin.LogSuccess("Starting Ping Processor ... done")

		lastPing = time.Now().Add(-constants.PING_CYCLE_LENGTH)

		outgoingPing := &ping.Ping{
			Issuer: ownpeer.INSTANCE,
		}
		outgoingPing.Sign()

		saltmanager.Events.UpdatePublicSalt.Attach(events.NewClosure(func(salt *salt.Salt) {
			outgoingPing.Sign()
		}))

		pingPeers(plugin, outgoingPing)

		ticker := time.NewTicker(constants.PING_PROCESS_INTERVAL)
	ticker:
		for {
			select {
			case <-daemon.ShutdownSignal:
				plugin.LogInfo("Stopping Ping Processor ...")

				break ticker
			case <-ticker.C:
				pingPeers(plugin, outgoingPing)
			}
		}

		plugin.LogSuccess("Stopping Ping Processor ... done")
	}
}

func pingPeers(plugin *node.Plugin, outgoingPing *ping.Ping) {
	if neighborhood.LIST_INSTANCE.Len() >= 1 {
		pingDelay := constants.PING_CYCLE_LENGTH / time.Duration(neighborhood.LIST_INSTANCE.Len())

		if lastPing.Add(pingDelay).Before(time.Now()) {
			chosenPeers := make(map[string]*peer.Peer)

			for i := 0; i < constants.PING_CONTACT_COUNT_PER_CYCLE; i++ {
				randomNeighborHoodPeer := neighborhood.LIST_INSTANCE.GetPeers()[rand.Intn(neighborhood.LIST_INSTANCE.Len())]

				if randomNeighborHoodPeer.GetIdentity().StringIdentifier != accountability.OwnId().StringIdentifier {
					chosenPeers[randomNeighborHoodPeer.GetIdentity().StringIdentifier] = randomNeighborHoodPeer
				}
			}

			for _, chosenPeer := range chosenPeers {
				go func(chosenPeer *peer.Peer) {
					if _, err := chosenPeer.Send(outgoingPing.Marshal(), types.PROTOCOL_TYPE_UDP, false); err != nil {
						plugin.LogDebug("error when sending ping to " + chosenPeer.String() + ": " + err.Error())
					} else {
						plugin.LogDebug("sent ping to " + chosenPeer.String())
					}
				}(chosenPeer)
			}

			lastPing = time.Now()
		}
	}
}
