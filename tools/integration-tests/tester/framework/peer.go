package framework

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/plugins/webapi/autopeering"
	"github.com/iotaledger/hive.go/identity"
)

// Peer represents a GoShimmer node inside the Docker network
type Peer struct {
	// name of the GoShimmer instance, Docker container and hostname
	name string
	// GoShimmer identity
	*identity.Identity

	// Web API of this peer
	*client.GoShimmerAPI

	// the DockerContainer that this peer is running in
	*DockerContainer

	chosen   []autopeering.Neighbor
	accepted []autopeering.Neighbor
}

// newPeer creates a new instance of Peer with the given information.
func newPeer(name string, identity *identity.Identity, dockerContainer *DockerContainer) *Peer {
	return &Peer{
		name:            name,
		Identity:        identity,
		GoShimmerAPI:    client.NewGoShimmerAPI(getWebApiBaseUrl(name), http.Client{Timeout: 30 * time.Second}),
		DockerContainer: dockerContainer,
	}
}

func (p *Peer) String() string {
	return fmt.Sprintf("Peer:{%s, %s, %s, %d}", p.name, p.ID().String(), p.BaseURL(), p.TotalNeighbors())
}

// TotalNeighbors returns the total number of neighbors the peer has.
func (p *Peer) TotalNeighbors() int {
	return len(p.chosen) + len(p.accepted)
}

// SetNeighbors sets the neighbors of the peer accordingly.
func (p *Peer) SetNeighbors(chosen, accepted []autopeering.Neighbor) {
	p.chosen = make([]autopeering.Neighbor, len(chosen))
	copy(p.chosen, chosen)

	p.accepted = make([]autopeering.Neighbor, len(accepted))
	copy(p.accepted, accepted)
}
