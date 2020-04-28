package tests

import (
	"testing"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/require"
)

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n *framework.Network) {
	err := n.Shutdown()
	require.NoError(t, err)
}
