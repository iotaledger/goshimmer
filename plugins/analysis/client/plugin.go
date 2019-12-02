package client

import (
	"encoding/hex"
	"net"
	"time"

	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/disconnectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/removenode"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/parameter"
)

var log = logger.NewLogger("Analysis-Client")

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Analysis Client", func() {
		shuttingDown := false

		for !shuttingDown {
			select {
			case <-daemon.ShutdownSignal:
				return

			default:
				if conn, err := net.Dial("tcp", parameter.NodeConfig.GetString(CFG_SERVER_ADDRESS)); err != nil {
					log.Debugf("Could not connect to reporting server: %s", err.Error())

					timeutil.Sleep(1 * time.Second)
				} else {
					managedConn := network.NewManagedConnection(conn)
					eventDispatchers := getEventDispatchers(managedConn)

					reportCurrentStatus(eventDispatchers)
					setupHooks(plugin, managedConn, eventDispatchers)

					shuttingDown = keepConnectionAlive(managedConn)
				}
			}
		}
	})
}

func getEventDispatchers(conn *network.ManagedConnection) *EventDispatchers {
	return &EventDispatchers{
		AddNode: func(nodeId []byte) {
			_, _ = conn.Write((&addnode.Packet{NodeId: nodeId}).Marshal())
		},
		RemoveNode: func(nodeId []byte) {
			_, _ = conn.Write((&removenode.Packet{NodeId: nodeId}).Marshal())
		},
		ConnectNodes: func(sourceId []byte, targetId []byte) {
			_, _ = conn.Write((&connectnodes.Packet{SourceId: sourceId, TargetId: targetId}).Marshal())
		},
		DisconnectNodes: func(sourceId []byte, targetId []byte) {
			_, _ = conn.Write((&disconnectnodes.Packet{SourceId: sourceId, TargetId: targetId}).Marshal())
		},
	}
}

func reportCurrentStatus(eventDispatchers *EventDispatchers) {
	if local.INSTANCE != nil {
		eventDispatchers.AddNode(local.INSTANCE.ID().Bytes())
	}

	reportChosenNeighbors(eventDispatchers)
}

func setupHooks(plugin *node.Plugin, conn *network.ManagedConnection, eventDispatchers *EventDispatchers) {
	// define hooks ////////////////////////////////////////////////////////////////////////////////////////////////////

	onDiscoverPeer := events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Info("onDiscoverPeer: " + hex.EncodeToString(ev.Peer.ID().Bytes()))
		eventDispatchers.AddNode(ev.Peer.ID().Bytes())
	})

	onDeletePeer := events.NewClosure(func(ev *discover.DeletedEvent) {
		log.Info("onDeletePeer: " + hex.EncodeToString(ev.Peer.ID().Bytes()))
		eventDispatchers.RemoveNode(ev.Peer.ID().Bytes())
	})

	onAddAcceptedNeighbor := events.NewClosure(func(ev *selection.PeeringEvent) {
		log.Info("onAddAcceptedNeighbor: " + hex.EncodeToString(ev.Peer.ID().Bytes()) + " - " + hex.EncodeToString(local.INSTANCE.ID().Bytes()))
		eventDispatchers.ConnectNodes(ev.Peer.ID().Bytes(), local.INSTANCE.ID().Bytes())
	})

	onRemoveNeighbor := events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Info("onRemoveNeighbor: " + hex.EncodeToString(ev.DroppedID.Bytes()) + " - " + hex.EncodeToString(local.INSTANCE.ID().Bytes()))
		eventDispatchers.DisconnectNodes(ev.DroppedID.Bytes(), local.INSTANCE.ID().Bytes())
	})

	onAddChosenNeighbor := events.NewClosure(func(ev *selection.PeeringEvent) {
		log.Info("onAddChosenNeighbor: " + hex.EncodeToString(local.INSTANCE.ID().Bytes()) + " - " + hex.EncodeToString(ev.Peer.ID().Bytes()))
		eventDispatchers.ConnectNodes(local.INSTANCE.ID().Bytes(), ev.Peer.ID().Bytes())
	})

	// setup hooks /////////////////////////////////////////////////////////////////////////////////////////////////////

	discover.Events.PeerDiscovered.Attach(onDiscoverPeer)
	discover.Events.PeerDeleted.Attach(onDeletePeer)
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
	if autopeering.Selection != nil {
		for _, chosenNeighbor := range autopeering.Selection.GetOutgoingNeighbors() {
			dispatchers.AddNode(chosenNeighbor.ID().Bytes())
			dispatchers.ConnectNodes(local.INSTANCE.ID().Bytes(), chosenNeighbor.ID().Bytes())
		}
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
