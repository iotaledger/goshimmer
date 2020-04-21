package client

import (
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/node"

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
		ticker := time.NewTicker(ReportInterval * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
				return

			case <-ticker.C:
				conn, err := net.Dial("tcp", config.Node.GetString(CfgServerAddress))
				if err != nil {
					log.Debugf("Could not connect to reporting server: %s", err.Error())
					continue
				}
				managedConn := network.NewManagedConnection(conn)
				eventDispatchers := getEventDispatchers(managedConn)
				reportHeartbeat(eventDispatchers)
			}
		}
	}, shutdown.PriorityAnalysis)
}

func getEventDispatchers(conn *network.ManagedConnection) *EventDispatchers {
	return &EventDispatchers{
		Heartbeat: func(packet *heartbeat.Packet) {
			var out strings.Builder
			for _, value := range packet.OutboundIDs {
				out.WriteString(hex.EncodeToString(value))
			}
			var in strings.Builder
			for _, value := range packet.InboundIDs {
				in.WriteString(hex.EncodeToString(value))
			}
			log.Debugw(
				"Heartbeat",
				"nodeId", hex.EncodeToString(packet.OwnID),
				"outboundIds", out.String(),
				"inboundIds", in.String(),
			)

			// Marshal() copies the content of packet, it doesn't modify it.
			data, err := packet.Marshal()
			if err != nil {
				log.Info(err, " - heartbeat message skipped")
				return
			}

			connLock.Lock()
			defer connLock.Unlock()
			if _, err = conn.Write(data); err != nil {
				log.Debugw("Error while writing to connection", "Description", err)
			}
		},
	}
}

func reportHeartbeat(dispatchers *EventDispatchers) {
	// Get own ID
	var nodeID []byte
	if local.GetInstance() != nil {
		// Doesn't copy the ID, take care not to modify underlying bytearray!
		nodeID = local.GetInstance().ID().Bytes()
	}

	// Get outboundIds (chosen neighbors)
	outgoingNeighbors := autopeering.Selection.GetOutgoingNeighbors()
	outboundIds := make([][]byte, len(outgoingNeighbors))
	for i, neighbor := range outgoingNeighbors {
		// Doesn't copy the ID, take care not to modify underlying bytearray!
		outboundIds[i] = neighbor.ID().Bytes()
	}

	// Get inboundIds (accepted neighbors)
	incomingNeighbors := autopeering.Selection.GetIncomingNeighbors()
	inboundIds := make([][]byte, len(incomingNeighbors))
	for i, neighbor := range incomingNeighbors {
		// Doesn't copy the ID, take care not to modify underlying bytearray!
		inboundIds[i] = neighbor.ID().Bytes()
	}

	packet := &heartbeat.Packet{OwnID: nodeID, OutboundIDs: outboundIds, InboundIDs: inboundIds}

	dispatchers.Heartbeat(packet)
}
