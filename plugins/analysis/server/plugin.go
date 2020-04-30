package server

import (
	"errors"
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
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the name of the analysis server plugin.
	PluginName = "Analysis-Server"

	// CfgServerPort defines the config flag of the analysis server TCP port.
	CfgServerPort = "analysis.server.port"

	// IdleTimeout defines the idle timeout.
	IdleTimeout = 10 * time.Second
	// StateHeartbeat defines the state of the heartbeat.
	StateHeartbeat = packet.HeartbeatPacketHeader
)

func init() {
	flag.Int(CfgServerPort, 0, "tcp port for incoming analysis packets")
}

var (
	// ErrInvalidPacketHeader defines an invalid package header error.
	ErrInvalidPacketHeader = errors.New("invalid packet header")
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
	server.Events.Start.Attach(events.NewClosure(func() {
		log.Infof("Starting Server (port %d) ... done", config.Node.GetInt(CfgServerPort))
	}))
	server.Events.Shutdown.Attach(events.NewClosure(func() {
		log.Info("Stopping Server ... done")
	}))
}

func run(_ *node.Plugin) {
	_ = daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		log.Infof("Starting Server (port %d) ... done", config.Node.GetInt(CfgServerPort))
		go server.Listen("0.0.0.0", config.Node.GetInt(CfgServerPort))
		<-shutdownSignal
		log.Info("Stopping Server ...")
		server.Shutdown()
		log.Info("Stopping Server ... done")
	}, shutdown.PriorityAnalysis)
}

// ConnectionState defines the type of a connection state as a byte
type ConnectionState = byte

// HandleConnection handles the given connection.
func HandleConnection(conn *network.ManagedConnection) {
	err := conn.SetTimeout(IdleTimeout)
	if err != nil {
		log.Errorf(err.Error())
	}

	var connectionState byte
	var receiveBuffer []byte

	var onDisconnect *events.Closure

	onReceiveData := events.NewClosure(func(data []byte) {
		processIncomingPacket(&connectionState, &receiveBuffer, conn, data)
	})
	onDisconnect = events.NewClosure(func() {
		conn.Events.ReceiveData.Detach(onReceiveData)
		conn.Events.Close.Detach(onDisconnect)
	})

	conn.Events.ReceiveData.Attach(onReceiveData)
	conn.Events.Close.Attach(onDisconnect)

	maxPacketsSize := getMaxPacketSize(packet.HeartbeatPacketMaxSize)

	go conn.Read(make([]byte, maxPacketsSize))
}

func getMaxPacketSize(packetSizes ...int) int {
	maxPacketSize := 0

	for _, packetSize := range packetSizes {
		if packetSize > maxPacketSize {
			maxPacketSize = packetSize
		}
	}

	return maxPacketSize
}

func processIncomingPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte) {
	var err error
	if *connectionState, *receiveBuffer, err = parsePackageHeader(data); err != nil {
		Events.Error.Trigger(err)
		conn.Close()
		return
	}

	switch *connectionState {
	case StateHeartbeat:
		processHeartbeatPacket(connectionState, receiveBuffer, conn, data)
	}
}

func parsePackageHeader(data []byte) (ConnectionState, []byte, error) {
	var connectionState ConnectionState
	var receiveBuffer []byte

	switch data[0] {
	case packet.HeartbeatPacketHeader:
		receiveBuffer = make([]byte, packet.HeartbeatPacketMaxSize)
		connectionState = StateHeartbeat
	default:
		return 0, nil, ErrInvalidPacketHeader
	}

	return connectionState, receiveBuffer, nil
}

func processHeartbeatPacket(_ *byte, _ *[]byte, conn *network.ManagedConnection, data []byte) {
	heartbeatPacket, err := packet.ParseHeartbeat(data)
	if err != nil {
		Events.Error.Trigger(err)
		conn.Close()
		return
	}
	Events.Heartbeat.Trigger(heartbeatPacket)
}
