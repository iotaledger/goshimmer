package tcp

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "net"
)

type PingConsumer = func(p *ping.Ping)

type RequestConsumer = func(req *request.Request)

type ResponseConsumer = func(peeringResponse *response.Response)

type IPErrorConsumer = func(ip net.IP, err error)

type ConnectionState = byte
