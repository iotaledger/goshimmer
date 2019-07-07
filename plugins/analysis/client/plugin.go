package client

import (
	"net"
	"time"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/disconnectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
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
	eventDispatchers.AddNode(accountability.OwnId().Identifier)

	reportChosenNeighbors(eventDispatchers)
}

func setupHooks(conn *network.ManagedConnection, eventDispatchers *EventDispatchers) {
	// define hooks ////////////////////////////////////////////////////////////////////////////////////////////////////

	onDiscoverPeer := events.NewClosure(func(p *peer.Peer) {
		go eventDispatchers.AddNode(p.Identity.Identifier)
	})

	onAddAcceptedNeighbor := events.NewClosure(func(p *peer.Peer) {
		eventDispatchers.ConnectNodes(p.Identity.Identifier, accountability.OwnId().Identifier)
	})

	onRemoveAcceptedNeighbor := events.NewClosure(func(p *peer.Peer) {
		eventDispatchers.DisconnectNodes(p.Identity.Identifier, accountability.OwnId().Identifier)
	})

	onAddChosenNeighbor := events.NewClosure(func(p *peer.Peer) {
		eventDispatchers.ConnectNodes(accountability.OwnId().Identifier, p.Identity.Identifier)
	})

	onRemoveChosenNeighbor := events.NewClosure(func(p *peer.Peer) {
		eventDispatchers.DisconnectNodes(accountability.OwnId().Identifier, p.Identity.Identifier)
	})

	// setup hooks /////////////////////////////////////////////////////////////////////////////////////////////////////

	knownpeers.INSTANCE.Events.Add.Attach(onDiscoverPeer)
	acceptedneighbors.INSTANCE.Events.Add.Attach(onAddAcceptedNeighbor)
	acceptedneighbors.INSTANCE.Events.Remove.Attach(onRemoveAcceptedNeighbor)
	chosenneighbors.INSTANCE.Events.Add.Attach(onAddChosenNeighbor)
	chosenneighbors.INSTANCE.Events.Remove.Attach(onRemoveChosenNeighbor)

	// clean up hooks on close /////////////////////////////////////////////////////////////////////////////////////////

	var onClose *events.Closure
	onClose = events.NewClosure(func() {
		knownpeers.INSTANCE.Events.Add.Detach(onDiscoverPeer)
		acceptedneighbors.INSTANCE.Events.Add.Detach(onAddAcceptedNeighbor)
		acceptedneighbors.INSTANCE.Events.Remove.Detach(onRemoveAcceptedNeighbor)
		chosenneighbors.INSTANCE.Events.Add.Detach(onAddChosenNeighbor)
		chosenneighbors.INSTANCE.Events.Remove.Detach(onRemoveChosenNeighbor)

		conn.Events.Close.Detach(onClose)
	})
	conn.Events.Close.Attach(onClose)
}

func reportChosenNeighbors(dispatchers *EventDispatchers) {
	for _, chosenNeighbor := range chosenneighbors.INSTANCE.Peers {
		dispatchers.AddNode(chosenNeighbor.Identity.Identifier)
	}
	for _, chosenNeighbor := range chosenneighbors.INSTANCE.Peers {
		dispatchers.ConnectNodes(accountability.OwnId().Identifier, chosenNeighbor.Identity.Identifier)
	}
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
