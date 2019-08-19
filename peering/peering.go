package peering

import (
	"log"
	"net"
	"time"

	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
)

type Peering struct {
	self       *identity.PrivateIdentity
	pinger     *Pinger
	knownPeers *PeerStore
}

func (p *Peering) OnPong(pkt *IngressPacket) {
	issuer := pkt.RemoteID
	pong := pkt.Message.GetPong()

	ping := p.pinger.PongReceived(pong)
	if ping != nil {
		// the receiver of the ping corresponds to the sender of the pong
		remote := ping.GetTo()
		peer := NewPeer(issuer, remote.GetIp(), uint16(remote.GetPort()))

		p.knownPeers.Add(peer)
	}
}

func (p *Peering) OnPing(pkt *IngressPacket) {
	ping := pkt.Message.GetPing()
	remote := ping.GetFrom()

	to := &net.UDPAddr{
		IP:   net.ParseIP(remote.GetIp()),
		Port: int(remote.GetPort()),
	}
	if err := p.pinger.Ping(to); err != nil {
		log.Println("Could not ping:", err)
	}
}

func NewPeering(id *identity.PrivateIdentity, sendPing func(to *net.UDPAddr, ping *pb.Ping) error) *Peering {
	return &Peering{
		self:       id,
		pinger:     NewPinger(30*time.Second, sendPing),
		knownPeers: NewPeerStore(),
	}
}

func (p *Peering) Start(addrs []*net.UDPAddr) {
	p.pinger.Start()

	for _, addr := range addrs {
		if err := p.pinger.Ping(addr); err != nil {
			log.Println("Could not ping:", err)
		}
	}
}

func (p *Peering) Stop() {
	p.pinger.Stop()
	p.pinger = nil
}
