package common

import (
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"

	"github.com/stretchr/testify/require"
)

// TestSynchronizationPersistence checks whether messages are relayed through the network,
// a node that joins later solidifies, stop and start this node again, and whether all messages
// are available on all nodes at the end (persistence).
func TestSynchronizationPersistence(t *testing.T) {
	initialPeers := 4
	n, err := f.CreateNetwork("common_TestSynchronizationPersistence", initialPeers, framework.CreateNetworkConfig{
		Faucet:      true,
		StartSynced: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	numMessages := 100

	// 1. issue data messages
	ids := tests.SendDataMessagesOnRandomPeer(t, n.Peers(), numMessages)
	log.Printf("Issuing %d messages to be synced... done\n", numMessages)

	// 2. spawn peer without knowledge of previous messages
	log.Println("Spawning new node to sync...")
	newPeer, err := n.CreatePeer(framework.GoShimmerConfig{})
	require.NoError(t, err)
	time.Sleep(2 * time.Second)
	err = n.DoManualPeeringAndWait()
	require.NoError(t, err)

	// 3. issue some messages on old peers so that new peer can solidify
	ids = tests.SendDataMessagesOnRandomPeer(t, n.Peers()[:initialPeers], 10, ids)

	// 4. check whether all issued messages are available on all nodes
	tests.CheckForMessageIDs(t, n.Peers(), ids, true)

	// 5. shut down newly added peer
	err = newPeer.Stop()
	require.NoError(t, err)
	log.Println("Stopping new node... done")

	// 6. let it startup again
	log.Println("Restarting new node to sync again...")
	err = newPeer.Start()
	require.NoError(t, err)
	// wait for peer to start
	time.Sleep(5 * time.Second)

	err = n.DoManualPeeringAndWait()
	require.NoError(t, err)

	// 7. issue some messages on old peers so that new peer can sync again
	ids = tests.SendDataMessagesOnRandomPeer(t, n.Peers()[:initialPeers], 10, ids)

	// 9. check whether all issued messages are available on all nodes
	tests.CheckForMessageIDs(t, n.Peers(), ids, true)
}
