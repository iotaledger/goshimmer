package client

import (
	"net"
	"time"

	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/disconnectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
	"github.com/iotaledger/hive.go/events"
)

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Analysis Client", func() {
		shuttingDown := false

		for !shuttingDown {
			select {
			case <-daemon.ShutdownSignal:
				return

			default:
				if conn, err := net.Dial("tcp", *SERVER_ADDRESS.Value); err != nil {
					plugin.LogDebug("Could not connect to reporting server: " + err.Error())

					timeutil.Sleep(1 * time.Second)
				} else {
					managedConn := network.NewManagedConnection(conn)
					eventDispatchers := getEventDispatchers(managedConn)

					reportCurrentStatus(eventDispatchers)
					setupHooks(managedConn, eventDispatchers)

					shuttingDown = keepConnectionAlive(managedConn)
				}
			}
		}
	})
}

func getEventDispatchers(conn *network.ManagedConnection) *EventDispatchers {
	return &EventDispatchers{
		AddNode: func(nodeId []byte) {
			conn.Write((&addnode.Packet{NodeId: nodeId}).Marshal())
		},
		ConnectNodes: func(sourceId []byte, targetId []byte) {
			conn.Write((&connectnodes.Packet{SourceId: sourceId, TargetId: targetId}).Marshal())
		},
		DisconnectNodes: func(sourceId []byte, targetId []byte) {
			conn.Write((&disconnectnodes.Packet{SourceId: sourceId, TargetId: targetId}).Marshal())
		},
	}
}

func reportCurrentStatus(eventDispatchers *EventDispatchers) {
	eventDispatchers.AddNode(accountability.OwnId().Identifier.Bytes())

	reportChosenNeighbors(eventDispatchers)
}

func setupHooks(conn *network.ManagedConnection, eventDispatchers *EventDispatchers) {
	// define hooks ////////////////////////////////////////////////////////////////////////////////////////////////////

	onDiscoverPeer := events.NewClosure(func(ev *discover.DiscoveredEvent) {
		go eventDispatchers.AddNode(ev.Peer.ID().Bytes())
	})

	onAddAcceptedNeighbor := events.NewClosure(func(ev *selection.PeeringEvent) {
		eventDispatchers.ConnectNodes(ev.Peer.ID().Bytes(), accountability.OwnId().Identifier.Bytes())
	})

	onRemoveNeighbor := events.NewClosure(func(ev *selection.DroppedEvent) {
		eventDispatchers.DisconnectNodes(ev.DroppedID.Bytes(), accountability.OwnId().Identifier.Bytes())
		eventDispatchers.DisconnectNodes(accountability.OwnId().Identifier.Bytes(), ev.DroppedID.Bytes())
	})

	onAddChosenNeighbor := events.NewClosure(func(ev *selection.PeeringEvent) {
		eventDispatchers.ConnectNodes(accountability.OwnId().Identifier.Bytes(), ev.Peer.ID().Bytes())
	})

	// setup hooks /////////////////////////////////////////////////////////////////////////////////////////////////////

	discover.Events.PeerDiscovered.Attach(onDiscoverPeer)
	selection.Events.IncomingPeering.Attach(onAddAcceptedNeighbor)
	selection.Events.OutgoingPeering.Attach(onAddChosenNeighbor)
	selection.Events.Dropped.Attach(onRemoveNeighbor)

	// clean up hooks on close /////////////////////////////////////////////////////////////////////////////////////////

	var onClose *events.Closure
	onClose = events.NewClosure(func() {
		discover.Events.PeerDiscovered.Detach(onDiscoverPeer)
		selection.Events.IncomingPeering.Detach(onAddAcceptedNeighbor)
		selection.Events.OutgoingPeering.Detach(onAddChosenNeighbor)
		selection.Events.Dropped.Detach(onRemoveNeighbor)

		conn.Events.Close.Detach(onClose)
	})
	conn.Events.Close.Attach(onClose)
}

func reportChosenNeighbors(dispatchers *EventDispatchers) {
	// for _, chosenNeighbor := range chosenneighbors.INSTANCE.Peers.GetMap() {
	// 	dispatchers.AddNode(chosenNeighbor.GetIdentity().Identifier)
	// }
	// for _, chosenNeighbor := range chosenneighbors.INSTANCE.Peers.GetMap() {
	// 	dispatchers.ConnectNodes(accountability.OwnId().Identifier, chosenNeighbor.GetIdentity().Identifier)
	// }
}

func keepConnectionAlive(conn *network.ManagedConnection) bool {
	go conn.Read(make([]byte, 1))

	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-daemon.ShutdownSignal:
			return true

		case <-ticker.C:
			if _, err := conn.Write((&ping.Packet{}).Marshal()); err != nil {
				return false
			}
		}
	}
}
