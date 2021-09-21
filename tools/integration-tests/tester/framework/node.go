package framework

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"

	"github.com/iotaledger/goshimmer/client"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Nodes is a slice of Node(s).
type Nodes = []*Node

// Node represents a GoShimmer node inside the Docker network
type Node struct {
	conf config.GoShimmer

	// GoShimmer identity
	*identity.Identity

	// Web API of this peer
	*client.GoShimmerAPI

	// the DockerContainer that this peer is running in
	*DockerContainer

	// the seed of the node's wallet
	*walletseed.Seed
}

// NewNode creates a new instance of Node with the given information.
// dockerContainer needs to be started in order to determine the container's (and therefore peer's) IP correctly.
func NewNode(conf config.GoShimmer, id *identity.Identity, dockerContainer *DockerContainer, seed *walletseed.Seed) *Node {
	return &Node{
		conf:            conf,
		Identity:        id,
		GoShimmerAPI:    client.NewGoShimmerAPI(getWebAPIBaseURL(conf.Name), client.WithHTTPClient(http.Client{Timeout: 5 * time.Second})),
		DockerContainer: dockerContainer,
		Seed:            seed,
	}
}

func (n *Node) String() string {
	return fmt.Sprintf("GoShimmer:{%s}", n.Name())
}

// Name returns the name of the node.
func (n *Node) Name() string {
	return n.conf.Name
}

// Config returns the configuration of the node.
func (n *Node) Config() config.GoShimmer {
	return n.conf
}

// Address returns the idx-th address of the wallet corresponding to the node.
func (n *Node) Address(idx int) ledgerstate.Address {
	if idx < 0 {
		panic("invalid index")
	}
	return n.Seed.Address(uint64(idx)).Address()
}

// Start starts the node and blocks until it is running.
func (n *Node) Start(ctx context.Context) error {
	if err := n.DockerContainer.Start(ctx); err != nil {
		return err
	}

	startContext, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := eventually(startContext, n.IsRunning, time.Second); err != nil {
		_, _ = n.Shutdown(ctx)
		return errors.Errorf("node %s did not come up in time", n)
	}
	return nil
}

// Restart restarts the node and blocks until it is running.
func (n *Node) Restart(ctx context.Context) error {
	if err := n.Stop(ctx); err != nil {
		return err
	}
	return n.Start(ctx)
}

// IsRunning returns true is the node is running.
func (n *Node) IsRunning() (bool, error) {
	err := n.HealthCheck()
	return err == nil, nil
}
