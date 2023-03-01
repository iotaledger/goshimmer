package tsc

import (
	"context"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestOrphanageTSC tests whether orphanage due to Time-Since-Acceptance works properly.
// This tests creates a network, spams some blocks so that all nodes see each other as active,
// and then splits the network into two partitions - one with majority weight. Blocks are issued on each partition and after that network is merged.
// After the network is merged, blocks issued in minority partition should be orphaned on nodes from that partition.
// Blocks from majority partition should become available on all nodes.
func TestOrphanageTSC(t *testing.T) {
	t.Skip("TSC is currently disabled in the codebase. This test will be re-enabled once TSC is re-enabled.")
	const tscThreshold = 30 * time.Second

	snapshotOptions := tests.OrphanageSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 4,
		framework.CreateNetworkConfig{
			StartSynced: false,
			Faucet:      false,
			Activity:    true,
			Autopeering: false,
			Snapshot:    snapshotOptions,
		}, tests.CommonSnapshotConfigFunc(t, snapshotInfo, func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
			conf.UseNodeSeedAsWalletSeed = true
			conf.TimeSinceConfirmationThreshold = tscThreshold
			conf.ValidatorActivityWindow = 10 * time.Minute
			return conf
		}))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	const delayBetweenDataMessages = 500 * time.Millisecond

	var (
		node1 = n.Peers()[0]
		node2 = n.Peers()[1]
		node3 = n.Peers()[2]
		node4 = n.Peers()[3]
	)

	log.Printf("Sending %d data blocks to the whole network", 10)
	tests.SendDataBlocksWithDelay(t, n.Peers(), 10, delayBetweenDataMessages)

	partition1 := []*framework.Node{node4}
	partition2 := []*framework.Node{node2, node3, node1}

	// split partitions
	err = n.CreatePartitionsManualPeering(ctx, partition1, partition2)
	require.NoError(t, err)

	// check consensus mana
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[0], tests.Mana(t, node1).Consensus)
	log.Printf("node1 (%s): %d", node1.ID().String(), tests.Mana(t, node1).Consensus)
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[1], tests.Mana(t, node2).Consensus)
	log.Printf("node2 (%s): %d", node2.ID().String(), tests.Mana(t, node2).Consensus)
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[2], tests.Mana(t, node3).Consensus)
	log.Printf("node3 (%s): %d", node3.ID().String(), tests.Mana(t, node3).Consensus)
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[3], tests.Mana(t, node4).Consensus)
	log.Printf("node4 (%s): %d", node4.ID().String(), tests.Mana(t, node4).Consensus)

	log.Printf("Sending %d data blocks on minority partition", 30)
	blocksToOrphan := tests.SendDataBlocksWithDelay(t, partition1, 30, delayBetweenDataMessages)
	log.Printf("Sending %d data blocks on majority partition", 10)
	blocksToConfirm := tests.SendDataBlocksWithDelay(t, partition2, 10, delayBetweenDataMessages)

	// merge partitions
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	// sleep 10 seconds to make sure that TSC threshold is exceeded
	time.Sleep(tscThreshold + time.Second)

	log.Printf("Sending %d data messages to make sure that all nodes share the same view", 30)
	tests.SendDataBlocksWithDelay(t, n.Peers(), 30, delayBetweenDataMessages)

	tests.RequireBlocksAvailable(t, n.Peers(), blocksToConfirm, time.Minute, tests.Tick, true)
	tests.RequireBlocksOrphaned(t, partition1, blocksToOrphan, time.Minute, tests.Tick)
}
