package common

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/hive.go/runtime/options"

	"github.com/mr-tron/base58"
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
		numSyncBlocks = 10 * initialPeers
	)
	snapshotOptions := tests.EqualSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), initialPeers, framework.CreateNetworkConfig{
		StartSynced: false,
		Snapshot:    snapshotOptions,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	// 1. issue data blocks
	log.Printf("Issuing %d blocks to sync...", numBlocks)
	ids := tests.SendDataBlocksWithDelay(t, n.Peers(), numBlocks, time.Millisecond*10)
	log.Println("Issuing blocks... done")

	// 2. spawn peer without knowledge of previous blocks
	log.Println("Spawning new node to sync...")

	cfg := createNewPeerConfig(t, snapshotOptions, 3)
	newPeer, err := n.CreatePeer(ctx, cfg)
	require.NoError(t, err)
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)
	log.Println("Spawning new node... done")

	// 3. issue some blocks on old peers so that new peer can solidify
	log.Printf("Issuing %d blocks on the %d initial peers...", numSyncBlocks, initialPeers)
	ids = tests.SendDataBlocksWithDelay(t, n.Peers()[:initialPeers], numSyncBlocks, time.Millisecond*10, ids)
	log.Println("Issuing blocks... done")

	// 4. check whether all issued blocks are available on to the new peer
	tests.RequireBlocksAvailable(t, n.Peers(), ids, time.Minute, tests.Tick)
	tests.RequireBlocksEqual(t, []*framework.Node{newPeer}, ids, time.Minute, tests.Tick)

	require.True(t, tests.Synced(t, newPeer))

	// 5. shut down newly added peer
	log.Println("Stopping new node...")
	require.NoError(t, newPeer.Stop(ctx))
	log.Println("Stopping new node... done")

	log.Printf("Issuing %d blocks and waiting until they have old tangle time...", numBlocks)
	ids = tests.SendDataBlocksWithDelay(t, n.Peers()[:initialPeers], numBlocks, 10*time.Millisecond, ids)
	// wait to assure that the new peer is actually out of sync when starting
	log.Printf("Sleeping %s to make sure new peer is out of sync when starting...", newPeer.Config().Protocol.BootstrapWindow.String())
	time.Sleep(newPeer.Config().Protocol.BootstrapWindow)
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

	// TODO: startup of a node is not yet supported. Therefore, the rest of this test is skipped.
	return

	// 7. issue some blocks on old peers so that new peer can sync again
	log.Printf("Issuing %d blocks on the %d initial peers...", numSyncBlocks, initialPeers)
	ids = tests.SendDataBlocksWithDelay(t, n.Peers()[:initialPeers], numSyncBlocks, 10*time.Millisecond, ids)
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

func TestConfirmBlock(t *testing.T) {
	snapshotOptions := tests.ConsensusSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: false,
		Snapshot:    snapshotOptions,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo, func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
		conf.UseNodeSeedAsWalletSeed = true
		return conf
	}))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	// Send a block and wait for it to be confirmed.
	peers := n.Peers()
	blockID, _ := tests.SendDataBlock(t, peers[0], []byte("Test"), 0)

	tests.TryAcceptBlock(t, peers, blockID, 30*time.Second, 100*time.Millisecond)
}

func createNewPeerConfig(t *testing.T, snapshotOptions []options.Option[snapshotcreator.Options], peerIndex int) config.GoShimmer {
	opt := snapshotcreator.NewOptions(snapshotOptions...)

	seedBytes, err := base58.Decode(opt.PeersSeedBase58[peerIndex])
	require.NoError(t, err)
	conf := framework.PeerConfig()
	conf.Seed = seedBytes
	conf.Protocol.Snapshot.Path = opt.FilePath
	// the new peer should use a shorter TangleTimeWindow than regular peers to go out of sync before them
	conf.Protocol.BootstrapWindow = 30 * time.Second
	return conf
}
