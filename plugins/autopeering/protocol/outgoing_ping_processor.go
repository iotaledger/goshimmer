package protocol

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/constants"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "github.com/iotaledger/goshimmer/plugins/gossip/neighbormanager"
    "math/rand"
    "net"
    "time"
)

func createOutgoingPingProcessor(plugin *node.Plugin) func() {
    return func() {
        plugin.LogInfo("Starting Ping Processor ...")
        plugin.LogSuccess("Starting Ping Processor ... done")
        
        outgoingPing := &ping.Ping{
            Issuer: &peer.Peer{
                Identity:            accountability.OWN_ID,
                PeeringProtocolType: types.PROTOCOL_TYPE_TCP,
                PeeringPort:         uint16(*parameters.UDP_PORT.Value),
                GossipProtocolType:  types.PROTOCOL_TYPE_TCP,
                GossipPort:          uint16(*parameters.UDP_PORT.Value),
                Address:             net.IPv4(0, 0, 0, 0),
                Salt:                saltmanager.PUBLIC_SALT,
            },
        }
        outgoingPing.Sign()

        saltmanager.Events.UpdatePublicSalt.Attach(func(salt *salt.Salt) {
            outgoingPing.Sign()
        })

        pingPeers(plugin, outgoingPing)

        ticker := time.NewTicker(constants.PING_RANDOM_PEERS_INTERVAL)
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
    chosenPeers := make(map[string]*peer.Peer)

    for i := 0; i < constants.PING_RANDOM_PEERS_COUNT; i++ {
        randomNeighborHoodPeer := peermanager.NEIGHBORHOOD_LIST[rand.Intn(len(peermanager.NEIGHBORHOOD_LIST))]

        nodeId := randomNeighborHoodPeer.Identity.StringIdentifier

        if !neighbormanager.ACCEPTED_NEIGHBORS.Contains(nodeId) && !neighbormanager.CHOSEN_NEIGHBORS.Contains(nodeId) &&
                nodeId != accountability.OWN_ID.StringIdentifier {
            chosenPeers[randomNeighborHoodPeer.Identity.StringIdentifier] = randomNeighborHoodPeer
        }
    }

    for _, chosenPeer := range chosenPeers {
        go func(chosenPeer *peer.Peer) {
            chosenPeer.Send(outgoingPing.Marshal(), false)

            plugin.LogDebug("sent ping to " + chosenPeer.String())
        }(chosenPeer)
    }
}