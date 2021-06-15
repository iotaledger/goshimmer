package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestFaucetPersistence sends funds by faucet request.
func TestFaucetPersistence(t *testing.T) {
	prevPoWDiff := framework.ParaPoWDifficulty
	framework.ParaPoWDifficulty = 0
	defer func() {
		framework.ParaPoWDifficulty = prevPoWDiff
	}()
	n, err := f.CreateNetwork("common_TestSynchronization", 5, framework.CreateNetworkConfig{
		Faucet:      true,
		StartSynced: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	peers := n.Peers()

	// wait for peers to change their state to synchronized
	time.Sleep(5 * time.Second)

	// master node sends funds to all peers in the network
	ids, addrBalance := tests.SendFaucetRequestOnRandomPeer(t, peers[1:], 10)

	// check whether all issued messages are available on all nodes
	tests.CheckForMessageIDs(t, n.Peers(), ids, true, 30*time.Second)

	// wait for transactions to be gossiped
	time.Sleep(2 * framework.DefaultUpperBoundNetworkDelay)

	// check ledger state
	tests.CheckBalances(t, peers[1:], addrBalance)

	// stop all nodes
	for _, peer := range n.Peers()[1:] {
		err = peer.Stop()
		require.NoError(t, err)
	}

	// start all nodes
	for _, peer := range n.Peers()[1:] {
		err = peer.Start()
		require.NoError(t, err)
	}

	// wait for peers to start
	time.Sleep(20 * time.Second)
	err = n.DoManualPeeringAndWait()
	require.NoError(t, err)

	// check whether all issued messages are available on all nodes
	tests.CheckForMessageIDs(t, n.Peers(), ids, false, 30*time.Second)

	// check ledger state
	tests.CheckBalances(t, peers[1:], addrBalance)
}
