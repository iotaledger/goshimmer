package server

import (
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/network/tcp"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/protocol"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the name of the analysis server plugin.
	PluginName = "Analysis-Server"

	// CfgAnalysisServerBindAddress defines the bind address of the analysis server.
	CfgAnalysisServerBindAddress = "analysis.server.bindAddress"

	// IdleTimeout defines the idle timeout of the read from the client's connection.
	IdleTimeout = 1 * time.Minute
)

func init() {
	flag.String(CfgAnalysisServerBindAddress, "0.0.0.0:16178", "the bind address of the analysis server")
}

var (
	// Plugin is the plugin instance of the analysis server plugin.
	Plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	server *tcp.TCPServer
	prot   *protocol.Protocol
	log    *logger.Logger
)

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	server = tcp.NewServer()

	server.Events.Connect.Attach(events.NewClosure(HandleConnection))
	server.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("error in server: %s", err.Error())
	}))
}

func run(_ *node.Plugin) {
	bindAddr := config.Node.GetString(CfgAnalysisServerBindAddress)
	addr, portStr, err := net.SplitHostPort(bindAddr)
	if err != nil {
		log.Fatal("invalid bind address in %s: %s", CfgAnalysisServerBindAddress, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatal("invalid port in %s: %s", CfgAnalysisServerBindAddress, err)
	}

	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
		defer log.Infof("Stopping %s ... done", PluginName)

		// connect protocol events to processors
		prot = protocol.New(packet.AnalysisMsgRegistry)
		wireUp(prot)

		go server.Listen(addr, port)

		<-shutdownSignal
		log.Info("Stopping Server ...")
		server.Shutdown()
	}, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// HandleConnection handles the given connection.
func HandleConnection(conn *network.ManagedConnection) {
	if err := conn.SetReadTimeout(IdleTimeout); err != nil {
		log.Warnw("Error setting read timeout; closing connection", "err", err)
		_ = conn.Close()
		return
	}
	onReceiveData := events.NewClosure(func(data []byte) {
		if _, err := prot.Read(data); err != nil {
			log.Debugw("Invalid message received; closing connection", "err", err)
			_ = conn.Close()
		}
	})
	conn.Events.ReceiveData.Attach(onReceiveData)
	// starts the protocol and reads from its connection
	go func() {
		buffer := make([]byte, 2048)
		_, err := conn.Read(buffer)
		if err != nil && err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Warnw("Read error", "err", err)
		}
		// always close the connection when we've stopped reading from it
		_ = conn.Close()
	}()
}

// wireUp connects the Received events of the protocol to the packet specific processor.
func wireUp(p *protocol.Protocol) {
	p.Events.Received[packet.MessageTypeHeartbeat].Attach(events.NewClosure(func(data []byte) {
		processHeartbeatPacket(data)
	}))
	p.Events.Received[packet.MessageTypeFPCHeartbeat].Attach(events.NewClosure(func(data []byte) {
		processFPCHeartbeatPacket(data)
	}))
	p.Events.Received[packet.MessageTypeMetricHeartbeat].Attach(events.NewClosure(func(data []byte) {
		processMetricHeartbeatPacket(data)
	}))
}

// processHeartbeatPacket parses the serialized data into a Heartbeat packet and triggers its event.
func processHeartbeatPacket(data []byte) {
	heartbeatPacket, err := packet.ParseHeartbeat(data)
	if err != nil {
		Events.Error.Trigger(err)
		return
	}
	Events.Heartbeat.Trigger(heartbeatPacket)
}

// processHeartbeatPacket parses the serialized data into a FPC Heartbeat packet and triggers its event.
func processFPCHeartbeatPacket(data []byte) {
	hb, err := packet.ParseFPCHeartbeat(data)
	if err != nil {
		Events.Error.Trigger(err)
		return
	}
	Events.FPCHeartbeat.Trigger(hb)
}

// processMetricHeartbeatPacket parses the serialized data into a Metric Heartbeat packet and triggers its event.
func processMetricHeartbeatPacket(data []byte) {
	hb, err := packet.ParseMetricHeartbeat(data)
	if err != nil {
		Events.Error.Trigger(err)
		return
	}
	Events.MetricHeartbeat.Trigger(hb)
}
