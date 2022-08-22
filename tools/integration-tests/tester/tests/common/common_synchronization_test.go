package common

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestCommonSynchronization checks whether blocks are relayed through the network,
// a node that joins later solidifies, stop and start this node again, and whether all blocks
// are available on all nodes at the end (persistence).
func TestCommonSynchronization(t *testing.T) {
	const (
		initialPeers  = 3
		numBlocks     = 100
		numSyncBlocks = 5 * initialPeers
	)
	snapshotInfo := tests.EqualSnapshotDetails

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), initialPeers, framework.CreateNetworkConfig{
		StartSynced: true,
		Snapshot:    snapshotInfo,
		PeerMaster:  true,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// 1. issue data blocks
	log.Printf("Issuing %d blocks to sync...", numBlocks)
	ids := tests.SendDataBlocks(t, n.Peers(), numBlocks)
	log.Println("Issuing blocks... done")

	// 2. spawn peer without knowledge of previous blocks
	log.Println("Spawning new node to sync...")

	cfg := createNewPeerConfig(t, snapshotInfo, 2)
	newPeer, err := n.CreatePeer(ctx, cfg)
	require.NoError(t, err)
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)
	log.Println("Spawning new node... done")

	// 3. issue some blocks on old peers so that new peer can solidify
	log.Printf("Issuing %d blocks on the %d initial peers...", numSyncBlocks, initialPeers)
	ids = tests.SendDataBlocks(t, n.Peers()[:initialPeers], numSyncBlocks, ids)
	log.Println("Issuing blocks... done")

	// 4. check whether all issued blocks are available on to the new peer
	tests.RequireBlocksAvailable(t, []*framework.Node{newPeer}, ids, time.Minute, tests.Tick)
	tests.RequireBlocksEqual(t, []*framework.Node{newPeer}, ids, time.Minute, tests.Tick)

	require.True(t, tests.Synced(t, newPeer))

	// 5. shut down newly added peer
	log.Println("Stopping new node...")
	require.NoError(t, newPeer.Stop(ctx))
	log.Println("Stopping new node... done")

	log.Printf("Issuing %d blocks and waiting until they have old tangle time...", numBlocks)
	ids = tests.SendDataBlocks(t, n.Peers()[:initialPeers], numBlocks, ids)
	// wait to assure that the new peer is actually out of sync when starting
	time.Sleep(newPeer.Config().BlockLayer.TangleTimeWindow)
	log.Println("Issuing blocks... done")

	// 6. let it startup again
	log.Println("Restarting new node to sync again...")
	err = newPeer.Start(ctx)
	require.NoError(t, err)
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)
	log.Println("Restarting node... done")

	// the node should not be in sync as all the block are outside its sync time window
	require.False(t, tests.Synced(t, newPeer))

	// 7. issue some blocks on old peers so that new peer can sync again
	log.Printf("Issuing %d blocks on the %d initial peers...", numSyncBlocks, initialPeers)
	ids = tests.SendDataBlocks(t, n.Peers()[:initialPeers], numSyncBlocks, ids)
	log.Println("Issuing blocks... done")

	// 9. check whether all issued blocks are available on to the new peer
	tests.RequireBlocksAvailable(t, []*framework.Node{newPeer}, ids, time.Minute, tests.Tick)
	tests.RequireBlocksEqual(t, []*framework.Node{newPeer}, ids, time.Minute, tests.Tick)

	// check that the new node is synced
	require.Eventuallyf(t,
		func() bool { return tests.Synced(t, newPeer) },
		tests.Timeout, tests.Tick,
		"the peer %s did not sync again after restart", newPeer)
}

func TestFirewall(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 2, framework.CreateNetworkConfig{
		StartSynced: true,
		Snapshot:    tests.EqualSnapshotDetails,
	}, func(peerIndex int, peerMaster bool, cfg config.GoShimmer) config.GoShimmer {
		if peerIndex == 0 {
			cfg.Gossip.BlocksRateLimit.Limit = 50
		}
		return cfg
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)
	peer1, peer2 := n.Peers()[0], n.Peers()[1]
	got1, err := peer1.GetPeerFaultinessCount(peer2.ID())
	require.NoError(t, err)
	assert.Equal(t, 0, got1)
	got2, err := peer2.GetPeerFaultinessCount(peer1.ID())
	require.NoError(t, err)
	assert.Equal(t, 0, got2)

	// Start spamming blocks from peer2 to peer1.
	for i := 0; i < 51; i++ {
		tests.SendDataBlock(t, peer2, []byte(fmt.Sprintf("Test %d", i)), i)
		require.NoError(t, err)
	}
	assert.Eventually(t, func() bool {
		got1, err = peer1.GetPeerFaultinessCount(peer2.ID())
		require.NoError(t, err)
		return got1 != 0
	}, tests.Timeout, tests.Tick)
	got2, err = peer2.GetPeerFaultinessCount(peer1.ID())
	require.NoError(t, err)
	assert.Equal(t, 0, got2)
}

func TestConfirmBlock(t *testing.T) {
	snapshotInfo := tests.ConsensusSnapshotDetails

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 2, framework.CreateNetworkConfig{
		StartSynced: true,
		Snapshot:    snapshotInfo,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo, func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
		conf.UseNodeSeedAsWalletSeed = true
		return conf
	}))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// Send a block and wait for it to be confirmed.
	peers := n.Peers()
	blkID, _ := tests.SendDataBlock(t, peers[0], []byte("Test"), 0)

	tests.TryConfirmBlock(t, peers[:], blkID, 30*time.Second, 100*time.Millisecond)
}

func createNewPeerConfig(t *testing.T, snapshotInfo framework.SnapshotInfo, peerIndex int) config.GoShimmer {
	seedBytes, err := base58.Decode(snapshotInfo.PeersSeedBase58[peerIndex])
	require.NoError(t, err)
	conf := framework.PeerConfig()
	conf.Seed = seedBytes
	conf.BlockLayer.Snapshot.File = snapshotInfo.FilePath
	// the new peer should use a shorter TangleTimeWindow than regular peers to go out of sync before them
	conf.BlockLayer.TangleTimeWindow = 30 * time.Second
	return conf
}
