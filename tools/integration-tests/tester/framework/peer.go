package framework

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/api"
)

type Peer struct {
	name string
	ip   net.IP
	*api.Api
	chosen   []api.Neighbor
	accepted []api.Neighbor
}

func NewPeer(name string, ip net.IP) *Peer {
	return &Peer{
		name: name,
		ip:   ip,
		Api:  api.New(getWebApiBaseUrl(ip), http.Client{Timeout: 30 * time.Second}),
	}
}

func (p *Peer) String() string {
	return fmt.Sprintf("Peer:{%s, %s, %s, %d}", p.name, p.ip.String(), p.BaseUrl, p.TotalNeighbors())
}

func (p *Peer) TotalNeighbors() int {
	return len(p.chosen) + len(p.accepted)
}

func (p *Peer) SetNeighbors(chosen, accepted []api.Neighbor) {
	p.chosen = make([]api.Neighbor, len(chosen))
	copy(p.chosen, chosen)

	p.accepted = make([]api.Neighbor, len(accepted))
	copy(p.accepted, accepted)
}

func getAvailablePeers() (peers []*Peer) {
	// get peer master
	if addr, err := net.LookupIP(hostnamePeerMaster); err != nil {
		fmt.Printf("Could not resolve %s\n", hostnamePeerMaster)
	} else {
		p := NewPeer(hostnamePeerMaster, addr[0])
		peers = append(peers, p)
	}

	// get peer replicas
	for i := 1; ; i++ {
		peerName := fmt.Sprintf("%s%d", hostnamePeerReplicaPrefix, i)
		if addr, err := net.LookupIP(peerName); err != nil {
			//fmt.Printf("Could not resolve %s\n", peerName)
			break
		} else {
			p := NewPeer(peerName, addr[0])
			peers = append(peers, p)
		}
	}
	return
}

func getWebApiBaseUrl(ip net.IP) string {
	return fmt.Sprintf("http://%s:%s", ip.String(), apiPort)
}
