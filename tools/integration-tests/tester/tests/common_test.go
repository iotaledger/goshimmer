package tests

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSynchronization checks whether messages are relayed through the network,
// a node that joins later solidifies, whether it is desyned after a restart
// and becomes synced again.
func TestSynchronization(t *testing.T) {
	config := framework.NetworkConfig{
		BootstrapInitialIssuanceTimePeriodSec: 40,
	}
	initalPeers := 4
	n, err := f.CreateNetwork("common_TestSynchronization", initalPeers, 2, config)
	require.NoError(t, err)
	defer ShutdownNetwork(t, n)

	// wait for peers to change their state to synchronized
	time.Sleep(5 * time.Second)

	numMessages := 100

	// 1. issue data messages
	ids := sendDataMessagesOnRandomPeer(t, n.Peers(), numMessages)

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// 2. spawn peer without knowledge of previous messages
	newPeer, err := n.CreatePeer(framework.GoShimmerConfig{})
	require.NoError(t, err)
	err = n.WaitForAutopeering(3)
	require.NoError(t, err)

	// 3. issue some messages on old peers so that new peer can solidify
	ids = sendDataMessagesOnRandomPeer(t, n.Peers()[:initalPeers], 10, ids)

	// wait for peer to solidify
	time.Sleep(10 * time.Second)

	// 4. check whether all issued messages are available on all nodes
	checkForMessageIds(t, n.Peers(), ids, true)

	// 5. shut down newly added peer
	err = newPeer.Stop()
	require.NoError(t, err)

	// 6. let it startup again
	err = newPeer.Start()
	require.NoError(t, err)
	// wait for peer to start
	time.Sleep(2 * time.Second)

	// 7. check that it is in state desynced
	resp, err := newPeer.Info()
	require.NoError(t, err)
	assert.Falsef(t, resp.Synced, "Peer %s should be desynced but is synced!", newPeer.String())

	// 8. issue some messages on old peers so that new peer can sync again
	ids = sendDataMessagesOnRandomPeer(t, n.Peers()[:initalPeers], 10, ids)
	// wait for peer to sync
	time.Sleep(10 * time.Second)

	// 9. newPeer becomes synced again
	resp, err = newPeer.Info()
	require.NoError(t, err)
	assert.Truef(t, resp.Synced, "Peer %s should be synced but is desynced!", newPeer.String())

	// 10. check whether all issued messages are available on all nodes
	checkForMessageIds(t, n.Peers(), ids, true)
}
