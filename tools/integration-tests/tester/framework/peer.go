package framework

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
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

	chosen   []jsonmodels.Neighbor
	accepted []jsonmodels.Neighbor
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
	return len(p.chosen) + len(p.accepted)
}

// SetNeighbors sets the neighbors of the peer accordingly.
func (p *Peer) SetNeighbors(chosen, accepted []jsonmodels.Neighbor) {
	p.chosen = make([]jsonmodels.Neighbor, len(chosen))
	copy(p.chosen, chosen)

	p.accepted = make([]jsonmodels.Neighbor, len(accepted))
	copy(p.accepted, accepted)
}
