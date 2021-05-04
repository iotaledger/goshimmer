package framework

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
)

// Peer represents a GoShimmer node inside the Docker network
type Peer struct {
	// name of the GoShimmer instance, Docker container and hostname
	name string
	ip   string
	// GoShimmer identity
	*identity.Identity

	// Web API of this peer
	*client.GoShimmerAPI

	// the DockerContainer that this peer is running in
	*DockerContainer

	// Seed
	*walletseed.Seed

	neighborsNumber int
}

// newPeer creates a new instance of Peer with the given information.
// dockerContainer needs to be started in order to determine the container's (and therefore peer's) IP correctly.
func newPeer(name string, identity *identity.Identity, dockerContainer *DockerContainer, seed *walletseed.Seed, network *Network) (*Peer, error) {
	// after container is started we can get its IP
	ip, err := dockerContainer.IP(network.name)
	if err != nil {
		return nil, err
	}

	return &Peer{
		name:            name,
		ip:              ip,
		Identity:        identity,
		GoShimmerAPI:    client.NewGoShimmerAPI(getWebAPIBaseURL(name), client.WithHTTPClient(http.Client{Timeout: 30 * time.Second})),
		DockerContainer: dockerContainer,
		Seed:            seed,
	}, nil
}

func (p *Peer) String() string {
	return fmt.Sprintf("Peer:{%s, %s, %s, %d}", p.name, p.ID().String(), p.BaseURL(), p.TotalNeighbors())
}

// TotalNeighbors returns the total number of neighbors the peer has.
func (p *Peer) TotalNeighbors() int {
	return p.neighborsNumber
}

// SetNeighborsNumber sets the number of neighbors of the peer.
func (p *Peer) SetNeighborsNumber(number int) {
	p.neighborsNumber = number
}

func (p *Peer) ToPeerModel() *peer.Peer {
	services := service.New()
	const defaultFPCPort = 10895
	services.Update(service.FPCKey, "TCP", defaultFPCPort)
	const defaultPeeringPort = 14626
	services.Update(service.PeeringKey, "TCP", defaultPeeringPort)
	const defaultGossipPort = 14666
	services.Update(service.GossipKey, "TCP", defaultGossipPort)
	return peer.NewPeer(p.Identity, net.ParseIP(p.ip), services)
}

func ToPeerModels(peers []*Peer) []*peer.Peer {
	models := make([]*peer.Peer, len(peers))
	for i, p := range peers {
		models[i] = p.ToPeerModel()
	}
	return models
}
