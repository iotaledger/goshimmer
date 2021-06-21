package autopeering

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

func TestAutopeeringNetworkSplit(t *testing.T) {
	const (
		numPeers      = 6
		numPartitions = 2
	)

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetworkWithPartitions(ctx, t.Name(), numPeers, numPartitions, framework.CreateNetworkConfig{
		StartSynced: true,
		Autopeering: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// test that nodes only have neighbors from same partition
	for _, partition := range n.Partitions() {
		for _, peer := range partition.Peers() {
			resp, err := peer.GetAutopeeringNeighbors(false)
			require.NoError(t, err)

			// check that all neighbors are indeed in the same partition
			for _, n := range append(resp.Accepted, resp.Chosen...) {
				assert.Containsf(t, partition.PeerIDs(), n.ID,
					"peer '%s' has a neighbor outside it's partition", peer)
			}
		}
	}

	err = n.DeletePartitions(ctx)
	require.NoError(t, err)

	// let them mingle and check that they all peer with each other
	err = n.WaitForAutopeering(ctx)
	require.NoError(t, err)
}
