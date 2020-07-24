package syncbeacon

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncBeacon checks that beacon nodes broadcast sync beacons
// and follower nodes use those payloads to determine if they are synced or not.
func TestSyncBeacon(t *testing.T) {
	initialPeers := 4
	n, err := f.CreateNetwork("syncbeacon_TestSyncBeacon", 0, 0)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// create sync beacon nodes
	var beaconPublicKeys []string
	for i := 0; i < initialPeers; i++ {
		peer, err := n.CreatePeer(framework.GoShimmerConfig{
			SyncBeacon:                  true,
			SyncBeaconBroadcastInterval: 5,
			SyncBeaconFollower:          false,
		})
		require.NoError(t, err)
		beaconPublicKeys = append(beaconPublicKeys, peer.PublicKey().String())
	}
	peers := n.Peers()
	err = n.WaitForAutopeering(3)
	require.NoError(t, err)

	// beacon follower node to follow all previous nodes
	peer, err := n.CreatePeer(framework.GoShimmerConfig{
		SyncBeaconFollower:          true,
		SyncBeaconFollowNodes:       strings.Join(beaconPublicKeys, ","),
		SyncBeaconMaxTimeOfflineSec: 15,
	})
	require.NoError(t, err)
	err = n.WaitForAutopeering(3)
	require.NoError(t, err)

	log.Println("Waiting...1/2")
	// wait for node to solidify beacon messages
	time.Sleep(30 * time.Second)
	log.Println("done waiting.")

	resp, err := peer.Info()
	require.NoError(t, err)
	assert.Truef(t, resp.Synced, "Peer %s should be synced but is desynced!", peer.String())

	// 2. shutdown all but 1 beacon peers.
	for _, p := range peers[:initialPeers-1] {
		_ = p.Stop()
	}

	// wait for peers to sync and broadcast
	log.Println("Waiting...2/2")
	time.Sleep(30 * time.Second)
	log.Println("done waiting.")

	// expect majority of nodes to not have broadcasted beacons. Hence should be desynced due to cleanup.
	resp, err = peer.Info()
	require.NoError(t, err)
	assert.Falsef(t, resp.Synced, "Peer %s should be desynced but is synced!", peer.String())
}
