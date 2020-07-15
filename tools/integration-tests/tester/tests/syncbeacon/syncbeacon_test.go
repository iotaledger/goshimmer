package syncbeacon

import (
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"
	"strings"
	"testing"
	"time"
)

// TestSyncBeacon checks that sync payloads are being gossiped through the network,
// and follower nodes are using those payloads to determine if they are synced or not.
func TestSyncBeacon(t *testing.T) {
	framework.ParaPoWDifficulty = 0
	initialPeers := 4
	n, err := f.CreateNetwork("syncbeacon_TestSyncBeacon", initialPeers, 2)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for peers to change their state to synchronized
	time.Sleep(10 * time.Second)

	peers := n.Peers()
	var beaconPublicKeys []string
	for _, peer := range peers {
		beaconPublicKeys = append(beaconPublicKeys, peer.PublicKey().String())
	}

	// 1. Follow all nodes as beacon nodes
	peer, err := n.CreatePeer(framework.GoShimmerConfig{
		SyncBeaconFollowNodes: strings.Join(beaconPublicKeys, ","),
	})

	// wait for peers to change their state to synchronized
	time.Sleep(10 * time.Second)

	// issue some messages on old peers so that new peer can solidify
	ids := tests.SendDataMessagesOnRandomPeer(t, n.Peers()[:initialPeers], 10)

	log.Println("Waiting...")
	// wait for beacon nodes to broadcast their sync status
	time.Sleep(40 * time.Second)
	log.Println("done waiting.")

	// expect all beacon nodes to be synced and send their statuses.
	// node is also synced internally. So it should be synced.
	resp, err := peer.Info()
	require.NoError(t, err)
	assert.Truef(t, resp.Synced, "Peer %s should be synced but is desynced!", peer.String())

	// 2. shutdown all but 1 beacon node
	for _, p := range peers[:len(peers) - 2] {
		p.Stop()
	}

	// send some messages to the still running nodes
	// last node is the test node.
	ids = tests.SendDataMessagesOnRandomPeer(t, n.Peers()[initialPeers - 2 : initialPeers - 1], 10, ids)

	// wait for peers to sync and broadcast
	log.Println("Waiting...")
	time.Sleep(40 * time.Second)
	log.Println("done waiting.")


	// expect majority of nodes to not have updated their status. Hence should be false.
	// even though this node should be synced internally wrt to anchor points, it should be desynced wrt to its beacons.
	resp, err = peer.Info()
	require.NoError(t, err)
	assert.Falsef(t, resp.Synced, "Peer %s should be desynced but is synced!", peer.String())
}