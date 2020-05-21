package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkSplit(t *testing.T) {
	n, err := f.CreateNetworkWithPartitions("autopeering_TestNetworkSplit", 6, 2, 2)
	require.NoError(t, err)
	defer ShutdownNetwork(t, n)

	// test that nodes only have neighbors from same partition
	for _, partition := range n.Partitions() {
		for _, peer := range partition.Peers() {
			resp, err := peer.GetNeighbors(false)
			require.NoError(t, err)

			// check that all neighbors are indeed in the same partition
			for _, n := range resp.Accepted {
				assert.Contains(t, partition.PeersMap(), n.ID)
			}
			for _, n := range resp.Chosen {
				assert.Contains(t, partition.PeersMap(), n.ID)
			}
		}
	}

	err = n.DeletePartitions()
	require.NoError(t, err)

	// let them mingle and check that they all peer with each other
	err = n.WaitForAutopeering(4)
	require.NoError(t, err)
}
