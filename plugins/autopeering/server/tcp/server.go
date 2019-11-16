package tcp

import (
	"math"
	"net"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/tcp"
	"github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/pkg/errors"
)

var server = tcp.NewServer()
var log = logger.NewLogger("Autopeering-TCPServer")

func ConfigureServer(plugin *node.Plugin) {
	serverAddress := parameter.NodeConfig.GetString(parameters.CFG_ADDRESS)
	serverPort := parameter.NodeConfig.GetInt(parameters.CFG_PORT)

	server.Events.Connect.Attach(events.NewClosure(HandleConnection))
	server.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("error in tcp server: %s", err.Error())
	}))

	server.Events.Start.Attach(events.NewClosure(func() {
		if serverAddress == "0.0.0.0" {
			log.Infof("Starting TCP Server (port %d) ... done", serverPort)
		} else {
			log.Infof("Starting TCP Server (%s:%d) ... done", serverAddress, serverPort)
		}
	}))
	server.Events.Shutdown.Attach(events.NewClosure(func() {
		log.Info("Stopping TCP Server ... done")
	}))
}

func RunServer(plugin *node.Plugin) {
	serverAddress := parameter.NodeConfig.GetString(parameters.CFG_ADDRESS)
	serverPort := parameter.NodeConfig.GetInt(parameters.CFG_PORT)

	daemon.BackgroundWorker("Autopeering TCP Server", func() {
		if serverAddress == "0.0.0.0" {
			log.Infof("Starting TCP Server (port %d) ...", serverPort)
		} else {
			log.Infof("Starting TCP Server (%s:%d) ...", serverAddress, serverPort)
		}

		server.Listen(parameter.NodeConfig.GetInt(parameters.CFG_PORT))
	})
}

func ShutdownServer(plugin *node.Plugin) {
	log.Info("Stopping TCP Server ...")

	server.Shutdown()
}

func HandleConnection(conn *network.ManagedConnection) {
	conn.SetTimeout(IDLE_TIMEOUT)

	var connectionState = STATE_INITIAL
	var receiveBuffer []byte
	var offset int

	conn.Events.ReceiveData.Attach(events.NewClosure(func(data []byte) {
		ProcessIncomingPacket(&connectionState, &receiveBuffer, conn, data, &offset)
	}))

	go conn.Read(make([]byte, int(math.Max(ping.MARSHALED_TOTAL_SIZE, math.Max(request.MARSHALED_TOTAL_SIZE, response.MARSHALED_TOTAL_SIZE)))))
}

func ProcessIncomingPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int) {
	if *connectionState == STATE_INITIAL {
		var err error
		if *connectionState, *receiveBuffer, err = parsePackageHeader(data); err != nil {
			Events.Error.Trigger(conn.RemoteAddr().(*net.TCPAddr).IP, err)

			conn.Close()

			return
		}

		*offset = 0

		switch *connectionState {
		case STATE_REQUEST:
			*receiveBuffer = make([]byte, request.MARSHALED_TOTAL_SIZE)
		case STATE_RESPONSE:
			*receiveBuffer = make([]byte, response.MARSHALED_TOTAL_SIZE)
		case STATE_PING:
			*receiveBuffer = make([]byte, ping.MARSHALED_TOTAL_SIZE)
		}
	}

	switch *connectionState {
	case STATE_REQUEST:
		processIncomingRequestPacket(connectionState, receiveBuffer, conn, data, offset)
	case STATE_RESPONSE:
		processIncomingResponsePacket(connectionState, receiveBuffer, conn, data, offset)
	case STATE_PING:
		processIncomingPingPacket(connectionState, receiveBuffer, conn, data, offset)
	}
}

func parsePackageHeader(data []byte) (byte, []byte, error) {
	var connectionState byte
	var receiveBuffer []byte

	switch data[0] {
	case request.MARSHALED_PACKET_HEADER:
		receiveBuffer = make([]byte, request.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_REQUEST
	case response.MARHSALLED_PACKET_HEADER:
		receiveBuffer = make([]byte, response.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_RESPONSE
	case ping.MARSHALED_PACKET_HEADER:
		receiveBuffer = make([]byte, ping.MARSHALED_TOTAL_SIZE)

		connectionState = STATE_PING
	default:
		return 0, nil, errors.New("invalid package header")
	}

	return connectionState, receiveBuffer, nil
}

func processIncomingRequestPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int) {
	remainingCapacity := int(math.Min(float64(request.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < request.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if req, err := request.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(conn.RemoteAddr().(*net.TCPAddr).IP, err)

			conn.Close()

			return
		} else {
			req.Issuer.SetConn(conn)
			req.Issuer.SetAddress(conn.RemoteAddr().(*net.TCPAddr).IP)

			conn.Events.Close.Attach(events.NewClosure(func() {
				req.Issuer.SetConn(nil)
			}))

			Events.ReceiveRequest.Trigger(req)
		}

		*connectionState = STATE_INITIAL

		if *offset+len(data) > request.MARSHALED_TOTAL_SIZE {
			ProcessIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset)
		}
	}
}

func processIncomingResponsePacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int) {
	remainingCapacity := int(math.Min(float64(response.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < response.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if res, err := response.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(conn.RemoteAddr().(*net.TCPAddr).IP, err)

			conn.Close()

			return
		} else {
			res.Issuer.SetConn(conn)
			res.Issuer.SetAddress(conn.RemoteAddr().(*net.TCPAddr).IP)

			conn.Events.Close.Attach(events.NewClosure(func() {
				res.Issuer.SetConn(nil)
			}))

			Events.ReceiveResponse.Trigger(res)
		}

		*connectionState = STATE_INITIAL

		if *offset+len(data) > response.MARSHALED_TOTAL_SIZE {
			ProcessIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset)
		}
	}
}

func processIncomingPingPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int) {
	remainingCapacity := int(math.Min(float64(ping.MARSHALED_TOTAL_SIZE-*offset), float64(len(data))))

	copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

	if *offset+len(data) < ping.MARSHALED_TOTAL_SIZE {
		*offset += len(data)
	} else {
		if ping, err := ping.Unmarshal(*receiveBuffer); err != nil {
			Events.Error.Trigger(conn.RemoteAddr().(*net.TCPAddr).IP, err)

			conn.Close()

			return
		} else {
			ping.Issuer.SetConn(conn)
			ping.Issuer.SetAddress(conn.RemoteAddr().(*net.TCPAddr).IP)

			conn.Events.Close.Attach(events.NewClosure(func() {
				ping.Issuer.SetConn(nil)
			}))

			Events.ReceivePing.Trigger(ping)
		}

		*connectionState = STATE_INITIAL

		if *offset+len(data) > ping.MARSHALED_TOTAL_SIZE {
			ProcessIncomingPacket(connectionState, receiveBuffer, conn, data[remainingCapacity:], offset)
		}
	}
}
