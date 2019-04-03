package tcp

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/network/tcp"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "github.com/pkg/errors"
    "math"
    "net"
    "strconv"
)

var server = tcp.NewServer()

func ConfigureServer(plugin *node.Plugin) {
    server.Events.Connect.Attach(HandleConnection)
    server.Events.Error.Attach(func(err error) {
        plugin.LogFailure("error in tcp server: " + err.Error())
    })
    server.Events.Start.Attach(func() {
        if *parameters.ADDRESS.Value == "0.0.0.0" {
            plugin.LogSuccess("Starting TCP Server (port " + strconv.Itoa(*parameters.TCP_PORT.Value) + ") ... done")
        } else {
            plugin.LogSuccess("Starting TCP Server (" + *parameters.ADDRESS.Value + ":" + strconv.Itoa(*parameters.TCP_PORT.Value) + ") ... done")
        }
    })
    server.Events.Shutdown.Attach(func() {
        plugin.LogSuccess("Stopping TCP Server ... done")
    })
}

func RunServer(plugin *node.Plugin) {
    daemon.BackgroundWorker(func() {
        if *parameters.ADDRESS.Value == "0.0.0.0" {
            plugin.LogInfo("Starting TCP Server (port " + strconv.Itoa(*parameters.TCP_PORT.Value) + ") ...")
        } else {
            plugin.LogInfo("Starting TCP Server (" + *parameters.ADDRESS.Value + ":" + strconv.Itoa(*parameters.TCP_PORT.Value) + ") ...")
        }

        server.Listen(*parameters.TCP_PORT.Value)
    })
}

func ShutdownServer(plugin *node.Plugin) {
    plugin.LogInfo("Stopping TCP Server ...")

    server.Shutdown()
}

func HandleConnection(conn *network.ManagedConnection) {
    conn.SetTimeout(IDLE_TIMEOUT)

    var connectionState = STATE_INITIAL
    var receiveBuffer []byte
    var offset int

    conn.Events.ReceiveData.Attach(func(data []byte) {
        ProcessIncomingPacket(&connectionState, &receiveBuffer, conn, data, &offset)
    })

    go conn.Read(make([]byte, int(math.Max(request.MARSHALLED_TOTAL_SIZE, response.MARSHALLED_TOTAL_SIZE))))
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
            *receiveBuffer = make([]byte, request.MARSHALLED_TOTAL_SIZE)
        case STATE_RESPONSE:
            *receiveBuffer = make([]byte, response.MARSHALLED_TOTAL_SIZE)
        }
    }

    switch *connectionState {
    case STATE_REQUEST:
        processIncomingRequestPacket(connectionState, receiveBuffer, conn, data, offset)
    case STATE_RESPONSE:
        processIncomingResponsePacket(connectionState, receiveBuffer, conn, data, offset)
    }
}

func parsePackageHeader(data []byte) (ConnectionState, []byte, error) {
    var connectionState ConnectionState
    var receivedData []byte

    switch data[0] {
    case request.MARSHALLED_PACKET_HEADER:
        receivedData = make([]byte, request.MARSHALLED_TOTAL_SIZE)

        connectionState = STATE_REQUEST
    case response.MARHSALLED_PACKET_HEADER:
        receivedData = make([]byte, response.MARSHALLED_TOTAL_SIZE)

        connectionState = STATE_RESPONSE
    default:
        return 0, nil, errors.New("invalid package header")
    }

    return connectionState, receivedData, nil
}

func processIncomingRequestPacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int) {
    remainingCapacity := int(math.Min(float64(request.MARSHALLED_TOTAL_SIZE - *offset), float64(len(data))))

    copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

    if *offset + len(data) < request.MARSHALLED_TOTAL_SIZE {
        *offset += len(data)
    } else {
        if peeringRequest, err := request.Unmarshal(*receiveBuffer); err != nil {
            Events.Error.Trigger(conn.RemoteAddr().(*net.TCPAddr).IP, err)

            conn.Close()

            return
        } else {
            peeringRequest.Issuer.Address = conn.RemoteAddr().(*net.TCPAddr).IP

            Events.ReceiveRequest.Trigger(conn, peeringRequest)
        }

        *connectionState = STATE_INITIAL

        if *offset + len(data) > request.MARSHALLED_TOTAL_SIZE {
            ProcessIncomingPacket(connectionState, receiveBuffer, conn, data[request.MARSHALLED_TOTAL_SIZE:], offset)
        }
    }
}

func processIncomingResponsePacket(connectionState *byte, receiveBuffer *[]byte, conn *network.ManagedConnection, data []byte, offset *int) {
    remainingCapacity := int(math.Min(float64(response.MARSHALLED_TOTAL_SIZE - *offset), float64(len(data))))

    copy((*receiveBuffer)[*offset:], data[:remainingCapacity])

    if *offset + len(data) < response.MARSHALLED_TOTAL_SIZE {
        *offset += len(data)
    } else {
        if peeringResponse, err := response.Unmarshal(*receiveBuffer); err != nil {
            Events.Error.Trigger(conn.RemoteAddr().(*net.TCPAddr).IP, err)

            conn.Close()

            return
        } else {
            peeringResponse.Issuer.Address = conn.RemoteAddr().(*net.TCPAddr).IP

            Events.ReceiveResponse.Trigger(conn, peeringResponse)
        }

        *connectionState = STATE_INITIAL

        if *offset + len(data) > response.MARSHALLED_TOTAL_SIZE {
            ProcessIncomingPacket(connectionState, receiveBuffer, conn, data[response.MARSHALLED_TOTAL_SIZE:], offset)
        }
    }
}
