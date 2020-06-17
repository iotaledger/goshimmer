package value

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/require"
)

// TestValueIotaPersistence issues messages on random peers, restarts them and checks for persistence after restart.
func TestValueIotaPersistence(t *testing.T) {
	n, err := f.CreateNetwork("valueIota_TestPersistence", 4, 2)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for peers to change their state to synchronized
	time.Sleep(5 * time.Second)

	// master node sends funds to all peers in the network
	txIds, addrBalance := tests.SendValueMessagesOnFaucet(t, n.Peers())

	// wait for messages to be gossiped
	time.Sleep(10 * time.Second)

	// check whether the first issued transaction is available on all nodes, and confirmed
	tests.CheckTransactions(t, n.Peers(), txIds, true)

	// check ledger state
	tests.CheckBalances(t, n.Peers(), addrBalance)

	// send value message randomly
	randomTxIds := tests.SendValueMessagesOnRandomPeer(t, n.Peers(), addrBalance, 10)
	txIds = append(txIds, randomTxIds...)

	// wait for messages to be gossiped
	time.Sleep(10 * time.Second)

	// check whether all issued transactions are persistently available on all nodes, and confirmed
	tests.CheckTransactions(t, n.Peers(), txIds, true)

	// check ledger state
	tests.CheckBalances(t, n.Peers(), addrBalance)

	// 3. stop all nodes
	for _, peer := range n.Peers() {
		err = peer.Stop()
		require.NoError(t, err)
	}

	// 4. start all nodes
	for _, peer := range n.Peers() {
		err = peer.Start()
		require.NoError(t, err)
	}

	// wait for peers to start
	time.Sleep(10 * time.Second)

	// check whether all issued transactions are persistently available on all nodes, and confirmed
	tests.CheckTransactions(t, n.Peers(), txIds, true)

	// 5. check ledger state
	tests.CheckBalances(t, n.Peers(), addrBalance)
}

// TestValueColoredPersistence issues colored tokens on random peers, restarts them and checks for persistence after restart.
func TestValueColoredPersistence(t *testing.T) {
	n, err := f.CreateNetwork("valueColor_TestPersistence", 4, 2)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for peers to change their state to synchronized
	time.Sleep(5 * time.Second)

	// master node sends funds to all peers in the network
	txIds, addrBalance := tests.SendValueMessagesOnFaucet(t, n.Peers())

	// wait for messages to be gossiped
	time.Sleep(10 * time.Second)

	// check whether the transactions are available on all nodes, and confirmed
	tests.CheckTransactions(t, n.Peers(), txIds, true)

	// check ledger state
	tests.CheckBalances(t, n.Peers(), addrBalance)

	// send funds around
	randomTxIds := tests.SendColoredValueMessagesOnRandomPeer(t, n.Peers(), addrBalance, 10)
	txIds = append(txIds, randomTxIds...)

	// wait for value messages to be gossiped
	time.Sleep(10 * time.Second)

	// check whether all issued transactions are persistently available on all nodes, and confirmed
	tests.CheckTransactions(t, n.Peers(), txIds, true)

	// check ledger state
	tests.CheckBalances(t, n.Peers(), addrBalance)

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
	time.Sleep(10 * time.Second)

	// check whether all issued transactions are persistently available on all nodes, and confirmed
	tests.CheckTransactions(t, n.Peers(), txIds, true)

	// 5. check ledger state
	tests.CheckBalances(t, n.Peers(), addrBalance)
}
