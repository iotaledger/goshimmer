package client

import (
	"encoding/hex"
	"net"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
)

var log *logger.Logger
var connLock sync.Mutex

func Run(plugin *node.Plugin) {
	log = logger.NewLogger("Analysis-Client")
	daemon.BackgroundWorker("Analysis Client", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-shutdownSignal:
				return

			case <-ticker.C:
				if conn, err := net.Dial("tcp", config.Node.GetString(CFG_SERVER_ADDRESS)); err != nil {
					log.Debugf("Could not connect to reporting server: %s", err.Error())

					timeutil.Sleep(1*time.Second, shutdownSignal)
				} else {
					managedConn := network.NewManagedConnection(conn)
					eventDispatchers := getEventDispatchers(managedConn)

					reportHeartbeat(eventDispatchers)
				}
			}
		}
	}, shutdown.PriorityAnalysis)
}

func getEventDispatchers(conn *network.ManagedConnection) *EventDispatchers {
	return &EventDispatchers{
		Heartbeat: func(packet *heartbeat.Packet) {
			out := ""
			for _, value := range packet.OutboundIDs {
				out += hex.EncodeToString(value)
			}
			in := ""
			for _, value := range packet.InboundIDs {
				in += hex.EncodeToString(value)
			}
			log.Debugw(
				"Heartbeat",
				"nodeId", hex.EncodeToString(packet.OwnID),
				"outboundIds", out,
				"inboundIds", in,
			)
			connLock.Lock()
			_, _ = conn.Write(packet.Marshal())
			connLock.Unlock()

		},
	}
}

func reportHeartbeat(dispatchers *EventDispatchers) {
	// Get own ID
	var nodeId []byte
	if local.GetInstance() != nil {
		nodeId = local.GetInstance().ID().Bytes()
	}

	// Get outboundIds (chosen neighbors)
	outgoingNeighbors := autopeering.Selection.GetOutgoingNeighbors()
	outboundIds := make([][]byte, len(outgoingNeighbors))
	for i, neighbor := range outgoingNeighbors {
		outboundIds[i] = neighbor.ID().Bytes()
	}

	// Get inboundIds (accepted neighbors)
	incomingNeighbors := autopeering.Selection.GetIncomingNeighbors()
	inboundIds := make([][]byte, len(incomingNeighbors))
	for i, neighbor := range incomingNeighbors {
		inboundIds[i] = neighbor.ID().Bytes()
	}

	packet := heartbeat.Packet{OwnID: nodeId, OutboundIDs: outboundIds, InboundIDs: inboundIds}

	dispatchers.Heartbeat(&packet)
}
