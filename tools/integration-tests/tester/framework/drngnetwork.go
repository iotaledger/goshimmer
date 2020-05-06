package framework

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/client"
	hive_ed25519 "github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

// DRNGNetwork represents a complete drand with GoShimmer network within Docker.
// Including an entry node, drand members and arbitrary many peers.
type DRNGNetwork struct {
	network *Network
	members []*Drand
	distKey []byte
}

// newDRNGNetwork returns a Network instance, creates its underlying Docker network and adds the tester container to the network.
func newDRNGNetwork(dockerClient *client.Client, name string, tester *DockerContainer) (*DRNGNetwork, error) {
	network, err := newNetwork(dockerClient, name, tester)
	if err != nil {
		return nil, err
	}
	return &DRNGNetwork{
		network: network,
	}, nil
}

// CreatePeer creates a new peer/GoShimmer node in the network and returns it.
func (n *DRNGNetwork) CreatePeer(c GoShimmerConfig, publicKey hive_ed25519.PublicKey) (*Peer, error) {
	name := n.network.namePrefix(fmt.Sprintf("%s%d", containerNameReplica, len(n.network.peers)))

	conf := c
	conf.Name = name
	conf.EntryNodeHost = n.network.namePrefix(containerNameEntryNode)
	conf.EntryNodePublicKey = n.network.entryNodePublicKey()
	conf.DisabledPlugins = disabledPluginsPeer

	// create Docker container
	container := NewDockerContainer(n.network.dockerClient)
	err := container.CreateGoShimmerPeer(conf)
	if err != nil {
		return nil, err
	}
	err = container.ConnectToNetwork(n.network.id)
	if err != nil {
		return nil, err
	}
	err = container.Start()
	if err != nil {
		return nil, err
	}

	peer := newPeer(name, identity.New(publicKey), container)
	n.network.peers = append(n.network.peers, peer)
	return peer, nil
}

// CreateMember creates a new member/drand node in the network and returns it.
// Passing leader true enables the leadership on the given peer.
func (n *DRNGNetwork) CreateMember(leader bool) (*Drand, error) {
	name := n.network.namePrefix(fmt.Sprintf("%s%d", containerNameDrand, len(n.members)))

	// create Docker container
	container := NewDockerContainer(n.network.dockerClient)
	err := container.CreateDrandMember(name, fmt.Sprintf("%s:8080", n.network.namePrefix(fmt.Sprintf("%s%d", containerNameReplica, len(n.members)))), leader)
	if err != nil {
		return nil, err
	}
	err = container.ConnectToNetwork(n.network.id)
	if err != nil {
		return nil, err
	}
	err = container.Start()
	if err != nil {
		return nil, err
	}

	member := newDrand(name, container)
	n.members = append(n.members, member)
	return member, nil
}

// Shutdown creates logs and removes network and containers.
// Should always be called when a network is not needed anymore!
func (n *DRNGNetwork) Shutdown() error {
	defer n.network.Shutdown()

	for _, p := range n.members {
		err := p.Stop()
		if err != nil {
			return err
		}
	}

	// retrieve logs
	for _, p := range n.members {
		logs, err := p.Logs()
		if err != nil {
			return err
		}
		err = createLogFile(p.name, logs)
		if err != nil {
			return err
		}
	}

	// remove containers
	for _, p := range n.members {
		err := p.Remove()
		if err != nil {
			return err
		}
	}

	return nil
}

// WaitForDKG waits until all members have concluded the DKG phase.
func (n *DRNGNetwork) WaitForDKG() error {
	log.Printf("Waiting for DKG...\n")
	defer log.Printf("Waiting for DKG... done\n")

	for i := dkgMaxTries; i > 0; i-- {

		//var dkey *drand.DistKeyResponse
		for _, m := range n.members {

			if dkey, err := m.Client.DistKey(m.name+":8000", false); err != nil {
				log.Printf("request error: %v\n", err)
			} else {
				n.SetDistKey(dkey.Key)
			}
		}

		if len(n.distKey) != 0 {
			log.Printf("DistKey: %v", hex.EncodeToString(n.distKey))
			return nil
		}

		log.Println("Not done yet. Try again in 5 seconds...")
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("DKG not successful")
}

// SetDistKey sets the distributed key.
func (n *DRNGNetwork) SetDistKey(key []byte) {
	n.distKey = key
}

// Peers returns the list of peers.
func (n *DRNGNetwork) Peers() []*Peer {
	return n.network.Peers()
}
