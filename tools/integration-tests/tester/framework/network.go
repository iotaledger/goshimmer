package framework

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/mr-tron/base58"
	"golang.org/x/sync/errgroup"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/manualpeering"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
)

// Network represents a complete GoShimmer network within Docker.
// Including an entry node and arbitrary many peers.
type Network struct {
	Id   string
	name string

	docker          *client.Client
	tester          *DockerContainer
	socatContainers []*DockerContainer

	entryNode *Node
	peers     []*Node

	partitions []*Partition
}

// NewNetwork returns a Network instance, creates its underlying Docker network and adds the tester container to the network.
func NewNetwork(ctx context.Context, dockerClient *client.Client, name string, tester *DockerContainer) (*Network, error) {
	// create Docker network
	resp, err := dockerClient.NetworkCreate(ctx, name, types.NetworkCreate{})
	if err != nil {
		return nil, err
	}

	// the tester container needs to join the Docker network in order to communicate with the peers
	err = tester.ConnectToNetwork(ctx, resp.ID)
	if err != nil {
		return nil, err
	}

	return &Network{
		Id:     resp.ID,
		name:   name,
		tester: tester,
		docker: dockerClient,
	}, nil
}

// Peers returns all available peers in the network.
func (n *Network) Peers() []*Node {
	peersCopy := make([]*Node, len(n.peers))
	copy(peersCopy, n.peers)
	return peersCopy
}

// CreatePeer creates and returns a new GoShimmer peer.
// It blocks until this peer has started.
func (n *Network) CreatePeer(ctx context.Context, conf config.GoShimmer) (*Node, error) {
	name := n.namePrefix(fmt.Sprintf("%s%d", containerNameReplica, len(n.peers)))
	peer, err := n.createNode(ctx, name, conf)
	if err != nil {
		return nil, err
	}

	n.peers = append(n.peers, peer)
	return peer, nil
}

// DeletePartitions deletes all partitions of the network.
// All nodes can communicate with the full network again.
func (n *Network) DeletePartitions(ctx context.Context) error {
	log.Println("Deleting partitions...")
	defer log.Println("Deleting partitions... done")

	var eg errgroup.Group
	for _, partition := range n.partitions {
		partition := partition // capture range variable
		eg.Go(func() error {
			return partition.deletePartition(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	n.partitions = nil
	return nil
}

// Partitions returns the network's partitions.
func (n *Network) Partitions() []*Partition {
	return n.partitions
}

// Split splits the existing network in given partitions.
func (n *Network) Split(ctx context.Context, partitions ...[]*Node) error {
	for _, peers := range partitions {
		if err := n.createPartition(ctx, peers); err != nil {
			return err
		}
	}
	// wait until pumba containers are started and block traffic between partitions
	time.Sleep(graceTimePumba)

	return nil
}

// DoManualPeering creates a complete network topology between nodes using manual peering.
// If the optional list of nodes is not provided all network peers are used.
// DoManualPeering blocks until all connections are established or the ctx has expired.
func (n *Network) DoManualPeering(ctx context.Context, nodes ...*Node) error {
	if len(nodes) == 0 {
		nodes = n.peers
	}

	if err := n.addManualPeers(nodes); err != nil {
		return errors.Wrap(err, "adding manual peers failed")
	}
	if err := n.waitForManualPeering(ctx, nodes); err != nil {
		return errors.Wrap(err, "manual peering failed")
	}
	return nil
}

// WaitForAutopeering blocks until a fully connected network of neighbors has been found.
func (n *Network) WaitForAutopeering(ctx context.Context) error {
	condition := func() (bool, error) {
		// connection graph of all the nodes
		connections := make(map[string]map[string]struct{}, len(n.peers))
		// query all nodes and add their neighbors to the connection graph
		for _, peer := range n.peers {
			resp, err := peer.GetAutopeeringNeighbors(false)
			if err != nil {
				return false, errors.Wrap(err, "client failed to return autopeering connections")
			}
			neighbors := make(map[string]struct{}, len(n.peers))
			connections[peer.ID().String()] = neighbors
			for _, neighbor := range append(resp.Chosen, resp.Accepted...) {
				neighbors[neighbor.ID] = struct{}{}
			}
		}
		return n.isConnected(connections), nil
	}

	log.Printf("Waiting for %d peers to connect...", len(n.peers))
	defer log.Println("Waiting for peers to connect... done")
	return eventually(ctx, condition, time.Second)
}

// WaitForPeerDiscovery waits until all peers have discovered each other.
func (n *Network) WaitForPeerDiscovery(ctx context.Context) error {
	condition := func() (bool, error) {
		for _, peer := range n.peers {
			resp, err := peer.GetAutopeeringNeighbors(true)
			if err != nil {
				return false, errors.Wrap(err, "client failed to return known peers")
			}
			if !containsPeers(n.peers, peer, resp.KnownPeers) {
				return false, nil
			}
		}
		return true, nil
	}

	log.Println("Waiting for complete peer discovery...")
	defer log.Println("Waiting for complete peer discovery... done")
	return eventually(ctx, condition, time.Second)
}

// Shutdown creates logs and removes network and containers.
// Should always be called when a network is not needed anymore.
func (n *Network) Shutdown(ctx context.Context) error {
	// save exit status of containers to check at end of shutdown process
	exitStatus := make(map[string]int)

	// stop the entry peer first
	if n.entryNode != nil {
		status, err := n.entryNode.Shutdown(ctx)
		if err != nil {
			return err
		}
		exitStatus[n.entryNode.Name()] = status
	}

	// stop all peers in parallel
	var eg errgroup.Group
	for _, peer := range n.peers {
		peer := peer // capture range variable
		eg.Go(func() error {
			status, err := peer.Shutdown(ctx)
			exitStatus[peer.Name()] = status
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// stop all socat containers in parallel.
	for _, sc := range n.socatContainers {
		container := sc // capture range variable
		eg.Go(func() error {
			err := container.Kill(ctx, "SIGTERM")
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// delete all partitions
	if err := n.DeletePartitions(ctx); err != nil {
		return err
	}

	// remove entryNode container
	if n.entryNode != nil {
		if err := n.entryNode.Remove(ctx); err != nil {
			return err
		}
	}

	// remove all peer containers in parallel
	for _, peer := range n.peers {
		peer := peer // capture range variable
		eg.Go(func() error {
			return peer.Remove(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// remove all socat containers in parallel.
	for _, sc := range n.socatContainers {
		container := sc // capture range variable
		eg.Go(func() error {
			return container.Remove(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// disconnect tester from network otherwise the network can't be removed
	if err := n.tester.DisconnectFromNetwork(ctx, n.Id); err != nil {
		return err
	}

	// remove network
	if err := n.docker.NetworkRemove(ctx, n.Id); err != nil {
		return err
	}

	// check exit codes of containers
	for name, status := range exitStatus {
		if status != 0 {
			return fmt.Errorf("container %s exited with code %d", name, status)
		}
	}
	return nil
}

func (n *Network) createNode(ctx context.Context, name string, conf config.GoShimmer) (*Node, error) {
	conf.Name = name

	nodeID, err := conf.CreateIdentity()
	if err != nil {
		return nil, err
	}

	// create wallet
	nodeSeed := walletseed.NewSeed()
	if conf.UseNodeSeedAsWalletSeed {
		nodeSeed = walletseed.NewSeed(conf.Seed)
	}
	if conf.Faucet.Enabled {
		nodeSeed = walletseed.NewSeed(GenesisSeedBytes)
	}

	// create Docker container
	container := NewDockerContainer(n.docker)
	if err := container.CreateNode(ctx, conf); err != nil {
		return nil, err
	}
	if err := container.ConnectToNetwork(ctx, n.Id); err != nil {
		return nil, err
	}

	node := NewNode(conf, nodeID, container, nodeSeed)
	if err := node.Start(ctx); err != nil {
		return nil, err
	}
	return node, nil
}

// creates socat container to access node's ports while debugging
func (n *Network) createSocatContainer(ctx context.Context, targetNode *Node, containerNum int) (*DockerContainer, error) {
	offset := 100 * containerNum

	container := NewDockerContainer(n.docker)

	portMapping := make(map[int]config.GoShimmerPort, len(config.GoShimmerPorts))
	for _, port := range config.GoShimmerPorts {
		portMapping[int(port)+offset] = port
	}

	err := container.createSocatContainer(ctx, "socat_"+targetNode.Name(), targetNode.Name(), portMapping)
	if err != nil {
		return nil, err
	}
	if err := container.ConnectToNetwork(ctx, n.Id); err != nil {
		return nil, err
	}

	err = container.Start(ctx)
	if err != nil {
		return nil, err
	}

	n.socatContainers = append(n.socatContainers, container)

	return container, err
}

// createPumba creates and starts a Pumba Docker container.
func (n *Network) createPumba(ctx context.Context, effectedNode *Node, targetIPs []string) (*DockerContainer, error) {
	name := effectedNode.Name() + containerNameSuffixPumba
	container := NewDockerContainer(n.docker)
	err := container.CreatePumba(ctx, name, effectedNode.Name(), targetIPs)
	if err != nil {
		return nil, err
	}
	err = container.Start(ctx)
	if err != nil {
		return nil, err
	}
	return container, nil
}

// createEntryNode creates the network's entry node.
func (n *Network) createEntryNode(ctx context.Context) error {
	if n.entryNode != nil {
		panic("entry node already present")
	}

	node, err := n.createNode(ctx, n.namePrefix(containerNameEntryNode), EntryNodeConfig())
	if err != nil {
		return err
	}

	n.entryNode = node
	return nil
}

func (n *Network) createPeers(ctx context.Context, numPeers int, networkConfig CreateNetworkConfig, cfgAlterFunc ...CfgAlterFunc) error {
	// create a peer conf from the network conf
	conf := PeerConfig()
	if networkConfig.StartSynced {
		conf.MessageLayer.StartSynced = true
	}
	if networkConfig.Autopeering {
		conf.AutoPeering.Enabled = true
		conf.AutoPeering.EntryNodes = []string{
			fmt.Sprintf("%s@%s:%d", base58.Encode(n.entryNode.Identity.PublicKey().Bytes()), n.entryNode.Name(), peeringPort),
		}
	}
	if networkConfig.Activity {
		conf.Activity.Enabled = true
	}

	// the first peer is the peer master, it uses a special conf
	if networkConfig.PeerMaster {
		masterConfig := conf
		if networkConfig.Faucet {
			masterConfig.Faucet.Enabled = true
		}

		if len(cfgAlterFunc) > 0 && cfgAlterFunc[0] != nil {
			masterConfig = cfgAlterFunc[0](0, true, masterConfig)
		}
		log.Printf("Starting peer master...")
		if _, err := n.CreatePeer(ctx, masterConfig); err != nil {
			return err
		}
		// peer master counts as peer
		numPeers--
	}
	log.Printf("Starting %d peers...", numPeers)
	for i := 0; i < numPeers; i++ {
		if len(cfgAlterFunc) > 0 && cfgAlterFunc[0] != nil {
			if _, err := n.CreatePeer(ctx, cfgAlterFunc[0](i, false, conf)); err != nil {
				return err
			}
			continue
		}
		if _, err := n.CreatePeer(ctx, conf); err != nil {
			return err
		}
	}
	log.Println("Starting peers... done")

	return nil
}

// addManualPeers instructs each node to add all the other nodes as peers.
func (n *Network) addManualPeers(nodes []*Node) error {
	log.Printf("Adding manual peers to %d nodes...", len(nodes))
	for i := range nodes {
		// connect to all other nodes
		var peers []*manualpeering.KnownPeerToAdd
		for j, node := range nodes {
			if i == j {
				continue
			}
			p := &manualpeering.KnownPeerToAdd{
				PublicKey: node.PublicKey(),
				Address:   fmt.Sprintf("%s:%d", node.Name(), gossipPort),
			}
			peers = append(peers, p)
		}

		if err := nodes[i].AddManualPeers(peers); err != nil {
			return errors.Wrap(err, "failed to add manual nodes via API")
		}
	}
	log.Println("Adding manual peers... done")
	return nil
}

func (n *Network) waitForManualPeering(ctx context.Context, nodes []*Node) error {
	condition := func() (bool, error) {
		for _, node := range nodes {
			peers, err := node.GetManualPeers()
			if err != nil {
				return false, errors.Wrap(err, "client failed to return manually connected peers")
			}
			for _, p := range peers {
				if p.ConnStatus != manualpeering.ConnStatusConnected {
					return false, nil
				}
			}
		}
		return true, nil
	}

	log.Printf("Waiting for %d nodes to connect to their manual peers...", len(nodes))
	defer log.Println("Waiting for nodes to connect to their manual peers... done")
	return eventually(ctx, condition, 10*time.Second)
}

// namePrefix returns the suffix prefixed with the name.
func (n *Network) namePrefix(suffix string) string {
	return fmt.Sprintf("%s-%s", n.name, suffix)
}

// createPartition creates a partition with the given peers.
// It starts a Pumba container for every peer that blocks traffic to all other partitions.
func (n *Network) createPartition(ctx context.Context, nodes []*Node) error {
	idSet := make(map[string]struct{})
	for _, peer := range nodes {
		idSet[peer.ID().String()] = struct{}{}
	}

	// block all traffic to all other nodes except in the current partition
	var targetIPs []string
	for _, peer := range n.peers {
		if _, ok := idSet[peer.ID().String()]; ok {
			continue
		}

		ip, err := peer.DockerContainer.IP(ctx, n.name)
		if err != nil {
			return errors.Wrap(err, "failed to get container's IP")
		}
		targetIPs = append(targetIPs, ip)
	}

	partitionName := n.namePrefix(fmt.Sprintf("partition_%d-", len(n.partitions)))

	// create pumba container for every node in the partition
	pumbas := make([]*DockerContainer, len(nodes))
	for i, p := range nodes {
		pumba, err := n.createPumba(ctx, p, targetIPs)
		if err != nil {
			return err
		}
		pumbas[i] = pumba
	}

	partition := &Partition{
		name:   partitionName,
		peers:  nodes,
		pumbas: pumbas,
	}
	n.partitions = append(n.partitions, partition)

	return nil
}

// isConnected checks whether connections describes a fully connected network of all peers.
func (n *Network) isConnected(connections map[string]map[string]struct{}) bool {
	if len(n.Partitions()) == 0 {
		return isConnected(n.Peers(), connections)
	}

	for _, partition := range n.Partitions() {
		if !isConnected(partition.Peers(), connections) {
			return false
		}
	}
	return true
}

func isConnected(nodes []*Node, connections map[string]map[string]struct{}) bool {
	if len(nodes) == 0 {
		return true
	}
	visited := make(map[string]struct{}, len(connections))

	// simple DFS to determine reachable nodes
	var u string
	stack := []string{nodes[0].ID().String()}
	for len(stack) > 0 {
		u, stack = stack[len(stack)-1], stack[:len(stack)-1]
		for v := range connections[u] {
			if _, ok := visited[v]; ok {
				continue
			}
			visited[v] = struct{}{}
			stack = append(stack, v)
		}
	}

	for _, node := range nodes {
		if _, ok := visited[node.ID().String()]; !ok {
			return false
		}
	}
	return true
}

func containsPeers(allPeers []*Node, peer *Node, neighbors []jsonmodels.Neighbor) bool {
	neighborMap := make(map[string]struct{})
	for i := range neighbors {
		neighborMap[neighbors[i].ID] = struct{}{}
	}
	for i := range allPeers {
		if peer == allPeers[i] {
			continue
		}
		if _, ok := neighborMap[allPeers[i].ID().String()]; !ok {
			return false
		}
	}
	return true
}
