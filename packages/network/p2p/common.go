package p2p

import (
	"net"
	"strconv"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

// GetAddress returns the address of the p2p service.
func GetAddress(p *peer.Peer) string {
	p2pEndpoint := p.Services().Get(service.P2PKey)
	if p2pEndpoint == nil {
		panic("peer does not support p2p Endpoint")
	}
	return net.JoinHostPort(p.IP().String(), strconv.Itoa(p2pEndpoint.Port()))
}
