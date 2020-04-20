package framework

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Network struct {
	id   string
	name string

	peers     []*Peer
	entryNode *DockerContainer
	tester    *DockerContainer

	dockerClient *client.Client
}

func newNetwork(dockerClient *client.Client, name string, tester *DockerContainer) *Network {
	// create Docker network
	resp, err := dockerClient.NetworkCreate(context.Background(), name, types.NetworkCreate{})
	if err != nil {
		panic(err)
	}

	tester.ConnectToNetwork(resp.ID)

	return &Network{
		id:           resp.ID,
		name:         name,
		peers:        nil,
		tester:       tester,
		dockerClient: dockerClient,
	}
}

// Peers returns all available peers in the network.
func (n *Network) Peers() []*Peer {
	return n.peers
}

// RandomPeer returns a random peer out of the list of peers.
func (n *Network) RandomPeer() *Peer {
	return n.peers[rand.Intn(len(n.peers))]
}

func (n *Network) createEntryNode() {
	n.entryNode = NewDockerContainer(n.dockerClient)
	n.entryNode.CreateGoShimmer(n.getNamePrefix(containerNameEntryNode), "")
	n.entryNode.ConnectToNetwork(n.id)
	n.entryNode.Start()
}

func (n *Network) CreatePeer() *Peer {
	name := n.getNamePrefix(fmt.Sprintf("%s%d", containerNameReplica, len(n.peers)))

	//TODO: generate identity
	container := NewDockerContainer(n.dockerClient)
	container.CreateGoShimmer(name, n.getNamePrefix(containerNameEntryNode))
	container.ConnectToNetwork(n.id)
	container.Start()

	peer := NewPeer(name, name, container)
	n.peers = append(n.peers, peer)
	return peer
}

func (n *Network) Shutdown() {
	// stop containers
	n.entryNode.Stop()
	for _, p := range n.peers {
		p.Stop()
	}

	// retrieve logs
	createLogFile(n.getNamePrefix(containerNameEntryNode), n.entryNode.Logs())
	for _, p := range n.peers {
		createLogFile(p.name, p.Logs())
	}

	// remove containers
	n.entryNode.Remove()
	for _, p := range n.peers {
		p.Remove()
	}

	// disconnect tester from network
	n.tester.DisconnectFromNetwork(n.id)

	// remove network
	err := n.dockerClient.NetworkRemove(context.Background(), n.id)
	if err != nil {
		panic(err)
	}
}

func (n *Network) getNamePrefix(suffix string) string {
	return fmt.Sprintf("%s-%s", n.name, suffix)
}

// WaitForAutopeering waits until all peers have reached a minimum amount of neighbors.
// Panics if this minimum is not reached after autopeeringMaxTries.
func (n *Network) WaitForAutopeering(minimumNeighbors int) {
	fmt.Printf("Waiting for autopeering...\n")
	defer fmt.Printf("Waiting for autopeering... done\n")

	maxTries := autopeeringMaxTries
	for maxTries > 0 {

		for _, p := range n.peers {
			if resp, err := p.GetNeighbors(false); err != nil {
				fmt.Printf("request error: %v\n", err)
			} else {
				p.SetNeighbors(resp.Chosen, resp.Accepted)
			}
		}

		// verify neighbor requirement
		min := 100
		total := 0
		for _, p := range n.peers {
			neighbors := p.TotalNeighbors()
			if neighbors < min {
				min = neighbors
			}
			total += neighbors
		}
		if min >= minimumNeighbors {
			fmt.Printf("Neighbors: min=%d avg=%.2f\n", min, float64(total)/float64(len(n.peers)))
			return
		}

		fmt.Println("Not done yet. Try again in 5 seconds...")
		time.Sleep(5 * time.Second)
		maxTries--
	}
	panic("Peering not successful.")
}
