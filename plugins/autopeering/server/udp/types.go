package udp

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "net"
)

type PingConsumer = func(p *ping.Ping)

type ConnectionPeeringRequestConsumer = func(request *request.Request)

type ConnectionPeeringResponseConsumer = func(peeringResponse *response.Response)

type IPErrorConsumer = func(ip net.IP, err error)