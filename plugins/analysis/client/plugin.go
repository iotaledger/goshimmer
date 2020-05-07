package client

import (
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the name of  the analysis client plugin.
	PluginName = "Analysis-Client"
	// CfgServerAddress defines the config flag of the analysis server address.
	CfgServerAddress = "analysis.client.serverAddress"
	// defines the report interval of the reporting in seconds.
	reportIntervalSec = 5
)

func init() {
	flag.String(CfgServerAddress, "ressims.iota.cafe:188", "tcp server for collecting analysis information")
}

var (
	// Plugin is the plugin instance of the analysis client plugin.
	Plugin   = node.NewPlugin(PluginName, node.Enabled, run)
	log      *logger.Logger
	connLock sync.Mutex
)

func run(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(reportIntervalSec * time.Second)
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
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panic(err)
	}
}

// EventDispatchers holds the Heartbeat function.
type EventDispatchers struct {
	// Heartbeat defines the Heartbeat function.
	Heartbeat func(heartbeat *packet.Heartbeat)
}

func getEventDispatchers(conn *network.ManagedConnection) *EventDispatchers {
	return &EventDispatchers{
		Heartbeat: func(hb *packet.Heartbeat) {
			var out strings.Builder
			for _, value := range hb.OutboundIDs {
				out.WriteString(hex.EncodeToString(value))
			}
			var in strings.Builder
			for _, value := range hb.InboundIDs {
				in.WriteString(hex.EncodeToString(value))
			}
			log.Debugw(
				"Heartbeat",
				"nodeID", hex.EncodeToString(hb.OwnID),
				"outboundIDs", out.String(),
				"inboundIDs", in.String(),
			)

			data, err := packet.NewHeartbeatMessage(hb)
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
	// get own ID
	var nodeID []byte
	if local.GetInstance() != nil {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		nodeID = local.GetInstance().ID().Bytes()
	}

	var outboundIDs [][]byte
	var inboundIDs [][]byte

	// get outboundIDs (chosen neighbors)
	outgoingNeighbors := autopeering.Selection().GetOutgoingNeighbors()
	outboundIDs = make([][]byte, len(outgoingNeighbors))
	for i, neighbor := range outgoingNeighbors {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		outboundIDs[i] = neighbor.ID().Bytes()
	}

	// get inboundIDs (accepted neighbors)
	incomingNeighbors := autopeering.Selection().GetIncomingNeighbors()
	inboundIDs = make([][]byte, len(incomingNeighbors))
	for i, neighbor := range incomingNeighbors {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		inboundIDs[i] = neighbor.ID().Bytes()
	}

	hb := &packet.Heartbeat{OwnID: nodeID, OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
	dispatchers.Heartbeat(hb)
}
