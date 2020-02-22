package client

import (
	"encoding/hex"
	"net"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/disconnectnodes"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
	"github.com/iotaledger/goshimmer/plugins/analysis/types/removenode"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
)

var log *logger.Logger
var connLock sync.Mutex

func Run(plugin *node.Plugin) {
	log = logger.NewLogger("Analysis-Client")
	daemon.BackgroundWorker("Analysis Client", func(shutdownSignal <-chan struct{}) {
		shuttingDown := false

		for !shuttingDown {
			select {
			case <-shutdownSignal:
				return

			default:
				if conn, err := net.Dial("tcp", config.Node.GetString(CFG_SERVER_ADDRESS)); err != nil {
					log.Debugf("Could not connect to reporting server: %s", err.Error())

					timeutil.Sleep(1*time.Second, shutdownSignal)
				} else {
					managedConn := network.NewManagedConnection(conn)
					eventDispatchers := getEventDispatchers(managedConn)

					reportCurrentStatus(eventDispatchers)
					setupHooks(plugin, managedConn, eventDispatchers)

					shuttingDown = keepConnectionAlive(managedConn, shutdownSignal)
				}
			}
		}
	}, shutdown.ShutdownPriorityAnalysis)
}

func getEventDispatchers(conn *network.ManagedConnection) *EventDispatchers {
	return &EventDispatchers{
		AddNode: func(nodeId []byte) {
			log.Debugw("AddNode", "nodeId", hex.EncodeToString(nodeId))
			connLock.Lock()
			_, _ = conn.Write((&addnode.Packet{NodeId: nodeId}).Marshal())
			connLock.Unlock()
		},
		RemoveNode: func(nodeId []byte) {
			log.Debugw("RemoveNode", "nodeId", hex.EncodeToString(nodeId))
			connLock.Lock()
			_, _ = conn.Write((&removenode.Packet{NodeId: nodeId}).Marshal())
			connLock.Unlock()
		},
		ConnectNodes: func(sourceId []byte, targetId []byte) {
			log.Debugw("ConnectNodes", "sourceId", hex.EncodeToString(sourceId), "targetId", hex.EncodeToString(targetId))
			connLock.Lock()
			_, _ = conn.Write((&connectnodes.Packet{SourceId: sourceId, TargetId: targetId}).Marshal())
			connLock.Unlock()
		},
		DisconnectNodes: func(sourceId []byte, targetId []byte) {
			log.Debugw("DisconnectNodes", "sourceId", hex.EncodeToString(sourceId), "targetId", hex.EncodeToString(targetId))
			connLock.Lock()
			_, _ = conn.Write((&disconnectnodes.Packet{SourceId: sourceId, TargetId: targetId}).Marshal())
			connLock.Unlock()
		},
	}
}

func reportCurrentStatus(eventDispatchers *EventDispatchers) {
	if local.GetInstance() != nil {
		eventDispatchers.AddNode(local.GetInstance().ID().Bytes())
	}

	reportKnownPeers(eventDispatchers)
	reportChosenNeighbors(eventDispatchers)
	reportAcceptedNeighbors(eventDispatchers)
}

func setupHooks(plugin *node.Plugin, conn *network.ManagedConnection, eventDispatchers *EventDispatchers) {
	// define hooks ////////////////////////////////////////////////////////////////////////////////////////////////////

	onDiscoverPeer := events.NewClosure(func(ev *discover.DiscoveredEvent) {
		eventDispatchers.AddNode(ev.Peer.ID().Bytes())
	})

	onDeletePeer := events.NewClosure(func(ev *discover.DeletedEvent) {
		eventDispatchers.RemoveNode(ev.Peer.ID().Bytes())
	})

	onAddChosenNeighbor := events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			eventDispatchers.ConnectNodes(local.GetInstance().ID().Bytes(), ev.Peer.ID().Bytes())
		}
	})

	onAddAcceptedNeighbor := events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			eventDispatchers.ConnectNodes(ev.Peer.ID().Bytes(), local.GetInstance().ID().Bytes())
		}
	})

	onRemoveNeighbor := events.NewClosure(func(ev *selection.DroppedEvent) {
		eventDispatchers.DisconnectNodes(ev.DroppedID.Bytes(), local.GetInstance().ID().Bytes())
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
		discover.Events.PeerDeleted.Detach(onDeletePeer)
		selection.Events.IncomingPeering.Detach(onAddAcceptedNeighbor)
		selection.Events.OutgoingPeering.Detach(onAddChosenNeighbor)
		selection.Events.Dropped.Detach(onRemoveNeighbor)

		conn.Events.Close.Detach(onClose)
	})
	conn.Events.Close.Attach(onClose)
}

func reportKnownPeers(dispatchers *EventDispatchers) {
	if autopeering.Discovery != nil {
		for _, peer := range autopeering.Discovery.GetVerifiedPeers() {
			dispatchers.AddNode(peer.ID().Bytes())
		}
	}
}

func reportChosenNeighbors(dispatchers *EventDispatchers) {
	if autopeering.Selection != nil {
		for _, chosenNeighbor := range autopeering.Selection.GetOutgoingNeighbors() {
			//dispatchers.AddNode(chosenNeighbor.ID().Bytes())
			dispatchers.ConnectNodes(local.GetInstance().ID().Bytes(), chosenNeighbor.ID().Bytes())
		}
	}
}

func reportAcceptedNeighbors(dispatchers *EventDispatchers) {
	if autopeering.Selection != nil {
		for _, acceptedNeighbor := range autopeering.Selection.GetIncomingNeighbors() {
			//dispatchers.AddNode(acceptedNeighbor.ID().Bytes())
			dispatchers.ConnectNodes(acceptedNeighbor.ID().Bytes(), local.GetInstance().ID().Bytes())
		}
	}
}

func keepConnectionAlive(conn *network.ManagedConnection, shutdownSignal <-chan struct{}) bool {
	go conn.Read(make([]byte, 1))

	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-shutdownSignal:
			return true

		case <-ticker.C:
			connLock.Lock()
			if _, err := conn.Write((&ping.Packet{}).Marshal()); err != nil {
				connLock.Unlock()
				return false
			}
			connLock.Unlock()
		}
	}
}
