package framework

import (
	"fmt"
	"net"
)

type Peer struct {
	name     string
	ip       net.IP
	api      string
	chosen   []Neighbor
	accepted []Neighbor
}

func NewPeer(name string, ip net.IP) *Peer {
	return &Peer{
		name: name,
		ip:   ip,
		api:  getWebApi(ip),
	}
}

func (p *Peer) String() string {
	return fmt.Sprintf("Peer:{%s, %s, %s, %d}", p.name, p.ip.String(), p.api, p.TotalNeighbors())
}

func (p *Peer) TotalNeighbors() int {
	return len(p.chosen) + len(p.accepted)
}

func (p *Peer) SetNeighbors(chosen, accepted []Neighbor) {
	p.chosen = make([]Neighbor, len(chosen))
	copy(p.chosen, chosen)

	p.accepted = make([]Neighbor, len(accepted))
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

func getWebApi(ip net.IP) string {
	return fmt.Sprintf("http://%s:%s/", ip.String(), apiPort)
}

type GetNeighborResponse struct {
	KnownPeers []Neighbor `json:"known,omitempty"`
	Chosen     []Neighbor `json:"chosen"`
	Accepted   []Neighbor `json:"accepted"`
	Error      string     `json:"error,omitempty"`
}

type Neighbor struct {
	ID        string        `json:"id"`        // comparable node identifier
	PublicKey string        `json:"publicKey"` // public key used to verify signatures
	Services  []PeerService `json:"services,omitempty"`
}

type PeerService struct {
	ID      string `json:"id"`      // ID of the service
	Address string `json:"address"` // network address of the service
}
