package server

import (
	"net"
	"strconv"
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

	// IdleTimeout defines the idle timeout.
	IdleTimeout = 10 * time.Second
)

func init() {
	flag.String(CfgAnalysisServerBindAddress, "0.0.0.0:16178", "the bind address of the analysis server")
}

var (
	// Plugin is the plugin instance of the analysis server plugin.
	Plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	server *tcp.TCPServer
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
	err := conn.SetTimeout(IdleTimeout)
	if err != nil {
		log.Errorf(err.Error())
	}

	// create new protocol instance
	p := protocol.New(conn, packet.AnalysisMsgRegistry)

	onReceiveData := events.NewClosure(func(data []byte) {
		// process incoming data in protocol.Receive()
		p.Receive(data)
	})

	var onDisconnect *events.Closure
	onDisconnect = events.NewClosure(func() {
		conn.Events.ReceiveData.Detach(onReceiveData)
		conn.Events.Close.Detach(onDisconnect)
	})

	conn.Events.ReceiveData.Attach(onReceiveData)
	conn.Events.Close.Attach(onDisconnect)

	// connect protocol events to processors
	wireUp(p)

	// starts the protocol and reads from its connection
	go p.Start()
}

// wireUp connects the Received events of the protocol to the packet specific processor
func wireUp(p *protocol.Protocol) {
	p.Events.Received[packet.MessageTypeHeartbeat].Attach(events.NewClosure(func(data []byte) {
		processHeartbeatPacket(data, p)
	}))
	p.Events.Received[packet.MessageTypeFPCHeartbeat].Attach(events.NewClosure(func(data []byte) {
		processFPCHeartbeatPacket(data, p)
	}))
}

// processHeartbeatPacket parses the serialized data into a Heartbeat packet and triggers its event
func processHeartbeatPacket(data []byte, p *protocol.Protocol) {
	heartbeatPacket, err := packet.ParseHeartbeat(data)
	if err != nil {
		Events.Error.Trigger(err)
		p.CloseConnection()
		return
	}
	Events.Heartbeat.Trigger(heartbeatPacket)
}

// processHeartbeatPacket parses the serialized data into a Heartbeat packet and triggers its event
func processFPCHeartbeatPacket(data []byte, p *protocol.Protocol) {
	hb, err := packet.ParseFPCHeartbeat(data)
	if err != nil {
		Events.Error.Trigger(err)
		p.CloseConnection()
		return
	}
	Events.FPCHeartbeat.Trigger(hb)
}
