package mana

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/require"
)

func TestManaPersistence(t *testing.T) {
	n, err := f.CreateNetwork("mana_TestPersistence", 1, 0, true)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for faucet to move funds and move mana
	time.Sleep(10 * time.Second)

	peers := n.Peers()

	info, err := peers[0].Info()
	require.NoError(t, err)
	manaBefore := info.Mana
	require.Greater(t, manaBefore.Access, 0.0)
	require.Greater(t, manaBefore.Consensus, 0.0)

	// stop all nodes. Expects mana to be saved successfully
	for _, peer := range n.Peers() {
		err = peer.Stop()
		require.NoError(t, err)
	}

	// start all nodes
	for _, peer := range peers {
		err = peer.Start()
		require.NoError(t, err)
	}

	// wait for peers to start
	time.Sleep(5 * time.Second)

	info, err = peers[0].Info()
	require.NoError(t, err)
	manaAfter := info.Mana
	require.Greater(t, manaAfter.Access, 0.0)
	require.Greater(t, manaAfter.Consensus, 0.0)
}
