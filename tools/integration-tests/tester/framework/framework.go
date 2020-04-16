// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with discovering peers,
// waiting for them to autopeer and offers easy access to the peers' web API
// and logs via Docker.
package framework

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/docker/docker/client"
)

// Framework is a wrapper encapsulating all peers
type Framework struct {
	peers     []*Peer
	dockerCli *client.Client
}

// New creates a new instance of Framework, gets all available peers within the Docker network and
// waits for them to autopeer.
// Panics if no peer is found.
func New() *Framework {
	fmt.Printf("Finding available peers...\n")

	cli, err := client.NewClient(
		"unix:///var/run/docker.sock",
		"",
		nil,
		nil,
	)
	if err != nil {
		fmt.Println("Could not create docker CLI client.")
		panic(err)
	}

	f := &Framework{
		dockerCli: cli,
		peers:     getAvailablePeers(cli),
	}

	if len(f.peers) == 0 {
		panic("Could not find any peers in Docker network.")
	}
	fmt.Printf("Finding available peers... done. Peers: %v\n", f.peers)

	fmt.Printf("Waiting for autopeering...\n")
	f.waitForAutopeering()
	fmt.Printf("Waiting for autopeering... done\n")

	return f
}

// waitForAutopeering waits until all peers have reached a minimum amount of neighbors.
// Panics if this minimum is not reached after autopeeringMaxTries.
func (f *Framework) waitForAutopeering() {
	maxTries := autopeeringMaxTries
	for maxTries > 0 {

		for _, p := range f.peers {
			if resp, err := p.GetNeighbors(false); err != nil {
				fmt.Printf("request error: %v\n", err)
			} else {
				p.SetNeighbors(resp.Chosen, resp.Accepted)
			}
		}

		// verify neighbor requirement
		min := 100
		total := 0
		for _, p := range f.peers {
			neighbors := p.TotalNeighbors()
			if neighbors < min {
				min = neighbors
			}
			total += neighbors
		}
		if min >= autopeeringMinimumNeighbors {
			fmt.Printf("Neighbors: min=%d avg=%.2f\n", min, float64(total)/float64(len(f.peers)))
			return
		}

		fmt.Println("Not done yet. Try again in 5 seconds...")
		time.Sleep(5 * time.Second)
		maxTries--
	}
	panic("Peering not successful.")
}

// Peers returns all available peers.
func (f *Framework) Peers() []*Peer {
	return f.peers
}

// RandomPeer returns a random peer out of the list of peers.
func (f *Framework) RandomPeer() *Peer {
	return f.peers[rand.Intn(len(f.peers))]
}
