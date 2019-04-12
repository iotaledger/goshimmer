package protocol

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
    "github.com/iotaledger/goshimmer/plugins/gossip/neighbormanager"
    "math/rand"
    "net"
    "time"
)

var lastPing time.Time

func createOutgoingPingProcessor(plugin *node.Plugin) func() {
    return func() {
        plugin.LogInfo("Starting Ping Processor ...")
        plugin.LogSuccess("Starting Ping Processor ... done")

        lastPing = time.Now().Add(-constants.PING_CYCLE_LENGTH)
        
        outgoingPing := &ping.Ping{
            Issuer: &peer.Peer{
                Identity:            accountability.OWN_ID,
                Address:             net.IPv4(0, 0, 0, 0),
                PeeringPort:         uint16(*parameters.PORT.Value),
                GossipPort:          uint16(*parameters.PORT.Value),
                Salt:                saltmanager.PUBLIC_SALT,
            },
        }
        outgoingPing.Sign()

        saltmanager.Events.UpdatePublicSalt.Attach(func(salt *salt.Salt) {
            outgoingPing.Sign()
        })

        pingPeers(plugin, outgoingPing)

        ticker := time.NewTicker(constants.PING_PROCESS_INTERVAL)
        ticker:
        for {
            select {
            case <- daemon.ShutdownSignal:
                plugin.LogInfo("Stopping Ping Processor ...")

                break ticker
            case <- ticker.C:
                pingPeers(plugin, outgoingPing)
            }
        }

        plugin.LogSuccess("Stopping Ping Processor ... done")
    }
}

func pingPeers(plugin *node.Plugin, outgoingPing *ping.Ping) {
    pingDelay := constants.PING_CYCLE_LENGTH / time.Duration(len(neighborhood.LIST_INSTANCE))

    if lastPing.Add(pingDelay).Before(time.Now()) {
        chosenPeers := make(map[string]*peer.Peer)

        for i := 0; i < constants.PING_CONTACT_COUNT_PER_CYCLE; i++ {
            randomNeighborHoodPeer := neighborhood.LIST_INSTANCE[rand.Intn(len(neighborhood.LIST_INSTANCE))]

            nodeId := randomNeighborHoodPeer.Identity.StringIdentifier

            if !neighbormanager.ACCEPTED_NEIGHBORS.Contains(nodeId) && !neighbormanager.CHOSEN_NEIGHBORS.Contains(nodeId) &&
                nodeId != accountability.OWN_ID.StringIdentifier {
                chosenPeers[randomNeighborHoodPeer.Identity.StringIdentifier] = randomNeighborHoodPeer
            }
        }

        for _, chosenPeer := range chosenPeers {
            go func(chosenPeer *peer.Peer) {
                chosenPeer.Send(outgoingPing.Marshal(), types.PROTOCOL_TYPE_UDP, false)

                plugin.LogDebug("sent ping to " + chosenPeer.String())
            }(chosenPeer)
        }

        lastPing = time.Now()
    }
}