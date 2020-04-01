package framework

import (
	"fmt"
	"math/rand"
	"time"
)

type Framework struct {
	peers []*Peer
}

func New() *Framework {
	fmt.Printf("Finding available peers...\n")

	f := &Framework{
		peers: getAvailablePeers(),
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

func (f *Framework) Peers() []*Peer {
	return f.peers
}

func (f *Framework) RandomPeer() *Peer {
	return f.peers[rand.Intn(len(f.peers))]
}
