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

// TestSyncBeacon checks that beacon nodes broadcast sync beacons
// and follower nodes use those payloads to determine if they are synced or not.
func TestSyncBeacon(t *testing.T) {
	initialPeers := 4
	n, err := f.CreateNetwork("syncbeacon_TestSyncBeacon", 0, 0)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	for i := 0; i < initialPeers; i++ {
		_, err := n.CreatePeer(framework.GoShimmerConfig{
			SyncBeaconPrimary:           true,
			SyncBeaconFollowNodes:       "",
			SyncBeaconBroadcastInterval: 20,
		})
		require.NoError(t, err)
	}
	err = n.WaitForAutopeering(3)
	require.NoError(t, err)

	peers := n.Peers()
	var beaconPublicKeys []string
	for _, peer := range peers {
		beaconPublicKeys = append(beaconPublicKeys, peer.PublicKey().String())
	}

	// follow all nodes as beacon nodes
	peer, err := n.CreatePeer(framework.GoShimmerConfig{
		SyncBeaconPrimary:           false,
		SyncBeaconFollowNodes:       strings.Join(beaconPublicKeys, ","),
		SyncBeaconBroadcastInterval: 20,
	})
	require.NoError(t, err)
	err = n.WaitForAutopeering(3)
	require.NoError(t, err)

	log.Println("Waiting...1/2")
	// wait for node to solidify beacon messages
	time.Sleep(60 * time.Second)
	log.Println("done waiting.")

	resp, err := peer.Info()
	require.NoError(t, err)
	assert.Truef(t, resp.Synced, "Peer %s should be synced but is desynced!", peer.String())

	// 2. shutdown all but 2 beacon peers.
	for _, p := range peers[:len(peers)-2] {
		_ = p.Stop()
	}

	// wait for peers to sync and broadcast
	log.Println("Waiting...2/2")
	time.Sleep(40 * time.Second)
	log.Println("done waiting.")

	// expect majority of nodes to not have broadcasted beacons. Hence should be desynced due to cleanup.
	resp, err = peer.Info()
	require.NoError(t, err)
	assert.Falsef(t, resp.Synced, "Peer %s should be desynced but is synced!", peer.String())
}
