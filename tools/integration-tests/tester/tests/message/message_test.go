package message

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/require"
)

// TestPersistence issues messages on random peers, restarts them and checks for persistence after restart.
func TestPersistence(t *testing.T) {
	n, err := f.CreateNetwork("message_TestPersistence", 4, 2)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for peers to change their state to synchronized
	time.Sleep(5 * time.Second)

	// 1. issue data messages
	ids := tests.SendDataMessagesOnRandomPeer(t, n.Peers(), 100)

	// wait for messages to be gossiped
	time.Sleep(10 * time.Second)

	// 2. check whether all issued messages are available on all nodes
	tests.CheckForMessageIds(t, n.Peers(), ids, true)

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

	// 5. check whether all issued messages are persistently available on all nodes
	tests.CheckForMessageIds(t, n.Peers(), ids, false)
}
