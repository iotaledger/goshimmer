package framework

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/client"
	"github.com/drand/drand/net"
	"golang.org/x/sync/errgroup"
)

// DRNGNetwork represents a complete drand with GoShimmer network within Docker.
// Including an entry node, drand members and arbitrary many peers.
type DRNGNetwork struct {
	*Network

	members []*Drand
	distKey []byte
}

// newDRNGNetwork returns a DRNGNetwork instance, creates its underlying Docker network and adds the tester container to the network.
func newDRNGNetwork(ctx context.Context, dockerClient *client.Client, name string, tester *DockerContainer) (*DRNGNetwork, error) {
	network, err := NewNetwork(ctx, dockerClient, name, tester)
	if err != nil {
		return nil, err
	}
	return &DRNGNetwork{
		Network: network,
	}, nil
}

// CreateMember creates a new member/drand node in the network and returns it.
// Passing leader true enables the leadership on the given peer.
func (n *DRNGNetwork) CreateMember(ctx context.Context, leader bool) (*Drand, error) {
	name := n.Network.namePrefix(fmt.Sprintf("%s%d", containerNameDrand, len(n.members)))

	// create Docker container
	container := NewDockerContainer(n.Network.docker)
	err := container.CreateDrandMember(ctx, name, fmt.Sprintf("%s:8080", n.Network.namePrefix(fmt.Sprintf("%s%d", containerNameReplica, len(n.members)))), leader)
	if err != nil {
		return nil, err
	}
	err = container.ConnectToNetwork(ctx, n.Network.Id)
	if err != nil {
		return nil, err
	}

	member := newDrand(name, container)
	err = container.Start(ctx)
	if err != nil {
		return nil, err
	}

	n.members = append(n.members, member)
	return member, nil
}

// Shutdown creates logs and removes network and containers.
// Should always be called when a network is not needed anymore!
func (n *DRNGNetwork) Shutdown(ctx context.Context) error {
	// stop all drand members in parallel
	var eg errgroup.Group
	for _, member := range n.members {
		member := member // capture range variable
		eg.Go(func() error {
			_, err := member.Shutdown(ctx, 10*time.Second)
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// remove containers
	for _, member := range n.members {
		member := member // capture range variable
		eg.Go(func() error {
			return member.Remove(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	return n.Network.Shutdown(ctx)
}

// WaitForDKG waits until all members have concluded the DKG phase.
func (n *DRNGNetwork) WaitForDKG(ctx context.Context) error {
	condition := func() (bool, error) {
		chainInfo, err := n.members[0].Client.ChainInfo(net.CreatePeer(n.members[0].name+":8000", false))
		if err != nil {
			return false, nil
		}
		distKey, err := chainInfo.PublicKey.MarshalBinary()
		if err != nil {
			return false, err
		}
		n.distKey = distKey
		return true, nil
	}

	log.Printf("Waiting for DKG...\n")
	defer log.Printf("Waiting for DKG... done\n")
	return eventually(ctx, condition, time.Second)
}
