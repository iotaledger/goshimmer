// Package framework provides integration test functionality for GoShimmer with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"context"
	"encoding/hex"
	"log"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/docker/docker/client"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

var (
	once     sync.Once
	instance *Framework
)

// Framework is a wrapper that provides the integration testing functionality.
type Framework struct {
	tester *DockerContainer
	docker *client.Client
}

// Instance returns the singleton Framework instance.
func Instance() (f *Framework, err error) {
	once.Do(func() {
		f, err = newFramework(context.Background())
		instance = f
	})

	return instance, err
}

// newFramework creates a new instance of Framework, creates a DockerClient
// and creates a DockerContainer for the tester container where the tests are running in.
func newFramework(ctx context.Context) (*Framework, error) {
	dockerClient, err := newDockerClient()
	if err != nil {
		return nil, err
	}

	// Since we are running within a container, the HOSTNAME environment variable defaults
	// to a shortened the container Id.
	tester, err := NewDockerContainerFromExisting(ctx, dockerClient, os.Getenv("HOSTNAME"))
	if err != nil {
		return nil, err
	}

	f := &Framework{
		docker: dockerClient,
		tester: tester,
	}
	return f, nil
}

// CfgAlterFunc is a function called with the given peer's index and its configuration
type CfgAlterFunc func(peerIndex int, cfg config.GoShimmer) config.GoShimmer

// CreateNetwork creates and returns a network that contains numPeers GoShimmer peers.
// It blocks until all peers are connected.
func (f *Framework) CreateNetwork(ctx context.Context, name string, numPeers int, conf CreateNetworkConfig, cfgAlterFunc ...CfgAlterFunc) (*Network, error) {
	network, err := f.CreateNetworkNoAutomaticManualPeering(ctx, name, numPeers, conf, cfgAlterFunc...)
	if err == nil && !conf.Autopeering {
		err = network.DoManualPeering(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "manual peering failed")
		}
	}

	return network, err
}

func (f *Framework) CreateNetworkNoAutomaticManualPeering(ctx context.Context, name string, numPeers int,
	conf CreateNetworkConfig, cfgAlterFunc ...CfgAlterFunc) (*Network, error) {
	network, err := NewNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	// an entry node is only required for autopeering
	if conf.Autopeering {
		if err = network.createEntryNode(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to create entry node")
		}
	}

	err = network.createPeers(ctx, numPeers, conf, cfgAlterFunc...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create peers")
	}

	// wait for peering to complete
	if conf.Autopeering {
		err = network.WaitForAutopeering(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "autopeering failed")
		}
		err = network.WaitForPeerDiscovery(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "peer discovery failed")
		}
	}

	return network, nil
}

// CreateNetworkWithPartitions creates and returns a network that contains numPeers GoShimmer nodes
// distributed over numPartitions partitions. It blocks until all peers are connected.
func (f *Framework) CreateNetworkWithPartitions(ctx context.Context, name string, numPeers, numPartitions int, conf CreateNetworkConfig, cfgAlterFunc ...CfgAlterFunc) (*Network, error) {
	network, err := NewNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	// make sure that autopeering is on
	conf.Autopeering = true

	// create an entry node with blocked traffic
	log.Println("Starting entry node...")
	if err = network.createEntryNode(ctx); err != nil {
		return nil, err
	}
	pumba, err := network.createPumba(ctx, network.entryNode, nil)
	if err != nil {
		return nil, err
	}
	// wait until pumba is started and the traffic is blocked
	time.Sleep(graceTimePumba)
	log.Println("Starting entry node... done")

	if err = network.createPeers(ctx, numPeers, conf, cfgAlterFunc...); err != nil {
		return nil, err
	}

	log.Printf("Creating %d partitions for %d peers...", numPartitions, numPeers)
	chunkSize := numPeers / numPartitions
	for i := 0; i < numPeers; i += chunkSize {
		end := i + chunkSize
		if end > numPeers {
			end = numPeers
		}
		err = network.createPartition(ctx, network.peers[i:end])
		if err != nil {
			return nil, err
		}
	}
	// wait until pumba containers are started and block traffic between partitions
	time.Sleep(graceTimePumba)
	log.Println("Creating partitions... done")

	// delete pumba for entry node
	err = pumba.Stop(ctx)
	if err != nil {
		return nil, err
	}
	err = pumba.Remove(ctx)
	if err != nil {
		return nil, err
	}

	err = network.WaitForAutopeering(ctx)
	if err != nil {
		return nil, err
	}

	return network, nil
}

// CreateDRNGNetwork creates and returns a network that contains numPeers GoShimmer peers
// out of which numMembers are part of the DRNG committee. It blocks until all peers are connected.
func (f *Framework) CreateDRNGNetwork(ctx context.Context, name string, numMembers int, numPeers int) (*DRNGNetwork, error) {
	drng, err := newDRNGNetwork(ctx, f.docker, name, f.tester)
	if err != nil {
		return nil, err
	}

	// create numMembers/drand nodes
	for i := 0; i < numMembers; i++ {
		leader := i == 0
		if _, err = drng.CreateMember(ctx, leader); err != nil {
			return nil, err
		}
	}

	err = drng.WaitForDKG(ctx)
	if err != nil {
		return nil, err
	}

	// create GoShimmer identities
	pubKeys := make([]ed25519.PublicKey, numPeers)
	privKeys := make([]ed25519.PrivateKey, numPeers)
	var drngCommittee []string
	for i := 0; i < numPeers; i++ {
		pubKeys[i], privKeys[i], err = ed25519.GenerateKey()
		if err != nil {
			return nil, err
		}

		if i < numMembers {
			drngCommittee = append(drngCommittee, pubKeys[i].String())
		}
	}

	conf := PeerConfig()
	conf.Activity.Enabled = true
	conf.DRNG.Enabled = true
	conf.DRNG.Custom.InstanceID = 111
	conf.DRNG.Custom.Threshold = 3
	conf.DRNG.Custom.DistributedPubKey = hex.EncodeToString(drng.distKey)
	conf.DRNG.Custom.CommitteeMembers = drngCommittee

	conf.MessageLayer.StartSynced = true

	// create numPeers/GoShimmer nodes
	for i := 0; i < numPeers; i++ {
		conf.Seed = privKeys[i].Seed().Bytes()
		if _, e := drng.CreatePeer(ctx, conf); e != nil {
			return nil, e
		}
	}

	err = drng.DoManualPeering(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "manual peering failed")
	}
	return drng, nil
}
