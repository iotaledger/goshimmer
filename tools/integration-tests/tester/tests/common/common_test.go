package common

import (
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"

	"github.com/stretchr/testify/require"
)

// TestSynchronization checks whether messages are relayed through the network,
// a node that joins later solidifies, whether it is desynced after a restart
// and becomes synced again.
func TestSynchronization(t *testing.T) {
	initialPeers := 4
	n, err := f.CreateNetworkWithMana("common_TestSynchronization", initialPeers, 2, framework.CreateNetworkConfig{
		Faucet:      true,
		Mana:        true,
		StartSynced: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for peers to change their state to synchronized
	time.Sleep(5 * time.Second)

	numMessages := 100

	// 1. issue data messages
	ids := tests.SendDataMessagesOnRandomPeer(t, n.Peers(), numMessages)
	log.Printf("Issuing %d messages to be synced... done\n", numMessages)
	// wait for messages to be gossiped and leaf the synced time windows
	time.Sleep(framework.ParaTangleTimeWindow)

	// 2. spawn peer without knowledge of previous messages
	log.Println("Spawning new node to sync...")
	newPeer, err := n.CreatePeerWithMana(framework.GoShimmerConfig{Mana: true})
	require.NoError(t, err)
	// when the node has mana it must also be peered

	// 3. issue some messages on old peers so that new peer can solidify
	ids = tests.SendDataMessagesOnRandomPeer(t, n.Peers()[:initialPeers], 10, ids)
	// wait for peer to solidify
	time.Sleep(30 * time.Second)

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
	err = n.WaitForAutopeering(2)
	require.NoError(t, err)

	// note: this check is too dependent on the initial time a node sends bootstrap messages
	// and therefore very error prone. Therefore it's not done for now.
	// 7. check that it is in state desynced
	//resp, err := newPeer.Info()
	//require.NoError(t, err)
	//assert.Falsef(t, resp.Synced, "Peer %s should be desynced but is synced!", newPeer.String())

	// 8. issue some messages on old peers so that new peer can sync again
	ids = tests.SendDataMessagesOnRandomPeer(t, n.Peers()[:initialPeers], 10, ids)
	// wait for peer to solidify
	time.Sleep(30 * time.Second)

	// 9. check whether all issued messages are available on all nodes
	tests.CheckForMessageIDs(t, n.Peers(), ids, true)
}
