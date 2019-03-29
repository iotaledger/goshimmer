package server

import (
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "net"
)

type ConnectionPeeringRequestConsumer = func(conn network.Connection, request *protocol.PeeringRequest)

type IPPeeringRequestConsumer = func(ip net.IP, request *protocol.PeeringRequest)

type IPErrorConsumer = func(ip net.IP, err error)
