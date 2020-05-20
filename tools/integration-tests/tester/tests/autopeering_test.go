package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetworkSplit(t *testing.T) {
	n, err := f.CreateNetworkWithPartitions("autopeering_TestNetworkSplit", 6, 2, 2)
	require.NoError(t, err)
	defer ShutdownNetwork(t, n)

	// TODO: test that nodes only have neighbors from same partition
	// then remove partitions and let them mingle and check that they all peer with each other

	fmt.Println("This is a test.")
}
