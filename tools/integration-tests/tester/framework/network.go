package framework

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	hive_ed25519 "github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

// Network represents a complete GoShimmer network within Docker.
// Including an entry node and arbitrary many peers.
type Network struct {
	id   string
	name string

	peers  []*Peer
	tester *DockerContainer

	entryNode         *DockerContainer
	entryNodeIdentity *identity.Identity

	dockerClient *client.Client
}

// newNetwork returns a Network instance, creates its underlying Docker network and adds the tester container to the network.
func newNetwork(dockerClient *client.Client, name string, tester *DockerContainer) *Network {
	// create Docker network
	resp, err := dockerClient.NetworkCreate(context.Background(), name, types.NetworkCreate{})
	if err != nil {
		panic(err)
	}

	// the tester container needs to join the Docker network in order to communicate with the peers
	tester.ConnectToNetwork(resp.ID)

	return &Network{
		id:           resp.ID,
		name:         name,
		tester:       tester,
		dockerClient: dockerClient,
	}
}

// createEntryNode creates the network's entry node.
func (n *Network) createEntryNode() {
	// create identity
	publicKey, privateKey, err := hive_ed25519.GenerateKey()
	if err != nil {
		panic(err)
	}

	n.entryNodeIdentity = identity.New(publicKey)
	seed := base64.StdEncoding.EncodeToString(ed25519.PrivateKey(privateKey.Bytes()).Seed())

	// create entry node container
	n.entryNode = NewDockerContainer(n.dockerClient)
	n.entryNode.CreateGoShimmerEntryNode(n.namePrefix(containerNameEntryNode), seed)
	n.entryNode.ConnectToNetwork(n.id)
	n.entryNode.Start()
}

// CreatePeer creates a new peer/GoShimmer node in the network and returns it.
func (n *Network) CreatePeer() *Peer {
	name := n.namePrefix(fmt.Sprintf("%s%d", containerNameReplica, len(n.peers)))

	// create identity
	publicKey, privateKey, err := hive_ed25519.GenerateKey()
	if err != nil {
		panic(err)
	}
	seed := base64.StdEncoding.EncodeToString(ed25519.PrivateKey(privateKey.Bytes()).Seed())

	// create Docker container
	container := NewDockerContainer(n.dockerClient)
	container.CreateGoShimmerPeer(name, seed, n.namePrefix(containerNameEntryNode), n.entryNodePublicKey())
	container.ConnectToNetwork(n.id)
	container.Start()

	peer := newPeer(name, identity.New(publicKey), container)
	n.peers = append(n.peers, peer)
	return peer
}

// Shutdown creates logs and removes network and containers.
// Should always be called when a network is not needed anymore!
func (n *Network) Shutdown() {
	// stop containers
	n.entryNode.Stop()
	for _, p := range n.peers {
		p.Stop()
	}

	// retrieve logs
	createLogFile(n.namePrefix(containerNameEntryNode), n.entryNode.Logs())
	for _, p := range n.peers {
		createLogFile(p.name, p.Logs())
	}

	// remove containers
	n.entryNode.Remove()
	for _, p := range n.peers {
		p.Remove()
	}

	// disconnect tester from network otherwise the network can't be removed
	n.tester.DisconnectFromNetwork(n.id)

	// remove network
	err := n.dockerClient.NetworkRemove(context.Background(), n.id)
	if err != nil {
		panic(err)
	}
}

// WaitForAutopeering waits until all peers have reached the minimum amount of neighbors.
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

// namePrefix returns the suffix prefixed with the name.
func (n *Network) namePrefix(suffix string) string {
	return fmt.Sprintf("%s-%s", n.name, suffix)
}

// entryNodePublicKey returns the entry node's public key encoded as base64
func (n *Network) entryNodePublicKey() string {
	return base64.StdEncoding.EncodeToString(n.entryNodeIdentity.PublicKey().Bytes())
}

// Peers returns all available peers in the network.
func (n *Network) Peers() []*Peer {
	return n.peers
}

// RandomPeer returns a random peer out of the list of peers.
func (n *Network) RandomPeer() *Peer {
	return n.peers[rand.Intn(len(n.peers))]
}
