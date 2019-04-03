package tcp

import (
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "net"
)

type ConnectionPeeringRequestConsumer = func(conn *network.ManagedConnection, request *request.Request)

type ConnectionPeeringResponseConsumer = func(conn *network.ManagedConnection, peeringResponse *response.Response)

type IPErrorConsumer = func(ip net.IP, err error)

type ConnectionState = byte
