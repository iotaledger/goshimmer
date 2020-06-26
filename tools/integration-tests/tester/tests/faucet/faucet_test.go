package faucet

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/require"
)

// TestFaucetPersistence sends funds by faucet request.
func TestFaucetPersistence(t *testing.T) {
	prevPoWDiff := framework.ParaPoWDifficulty
	framework.ParaPoWDifficulty = 0
	defer func() {
		framework.ParaPoWDifficulty = prevPoWDiff
	}()
	n, err := f.CreateNetwork("faucet_TestPersistence", 5, 2)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	peers := n.Peers()

	// wait for peers to change their state to synchronized
	time.Sleep(5 * time.Second)

	// master node sends funds to all peers in the network
	ids, addrBalance := tests.SendFaucetRequestOnRandomPeer(t, peers[1:], 10)

	// wait for messages to be gossiped
	time.Sleep(2 * valuetransfers.DefaultAverageNetworkDelay)

	// check whether all issued messages are available on all nodes
	tests.CheckForMessageIds(t, n.Peers(), ids, true)

	// wait for transactions to be gossiped
	time.Sleep(2 * valuetransfers.DefaultAverageNetworkDelay)

	// check ledger state
	tests.CheckBalances(t, peers[1:], addrBalance)

	// stop all nodes
	for _, peer := range n.Peers() {
		err = peer.Stop()
		require.NoError(t, err)
	}

	// start all nodes
	for _, peer := range n.Peers() {
		err = peer.Start()
		require.NoError(t, err)
	}

	// wait for peers to start
	time.Sleep(20 * time.Second)

	// check whether all issued messages are available on all nodes
	tests.CheckForMessageIds(t, n.Peers(), ids, true)

	// check ledger state
	tests.CheckBalances(t, peers[1:], addrBalance)
}
