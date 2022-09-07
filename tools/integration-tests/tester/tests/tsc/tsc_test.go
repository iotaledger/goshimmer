package tsc

import (
	"context"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestOrphanageTSC tests whether consensus is able to resolve a simple double spend.
// We spawn a network of 2 nodes containing 40% and 20% of consensus mana respectively,
// let them both issue conflicting transactions, and assert that the transaction
// issued by the 40% node gains a high GoF while the other one gets "none" GoF over time as the 20% consensus mana
// node puts its weight to the 40% issued tx making it reach 60% AW and hence high GoF.
// The genesis seed contains 800000 tokens which we will use to issue conflicting transactions from both nodes.
func TestOrphanageTSC(t *testing.T) {

	snapshotInfo := tests.OrphanageSnapshotDetails

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetworkNoAutomaticManualPeering(ctx, "test_orphanage_tsc", 4,
		framework.CreateNetworkConfig{
			StartSynced: true,
			Faucet:      false,
			Activity:    true,
			Autopeering: false,
			PeerMaster:  false,
			Snapshot:    snapshotInfo,
		}, tests.CommonSnapshotConfigFunc(t, snapshotInfo, func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
			conf.UseNodeSeedAsWalletSeed = true
			conf.TimeSinceConfirmationThreshold = 10 * time.Second
			return conf
		}))

	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	const delayBetweenDataMessages = 100 * time.Millisecond

	var (
		node1 = n.Peers()[0]
		node2 = n.Peers()[1]
		node3 = n.Peers()[2]
		node4 = n.Peers()[3]
	)

	// merge partitions
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	t.Logf("Sending %d data blocks to the whole network", 50)
	tests.SendDataBlocks(t, n.Peers(), 50)

	partition1 := []*framework.Node{node4}
	partition2 := []*framework.Node{node2, node3, node1}

	// split partitions
	err = n.CreatePartitionsManualPeering(ctx, partition1, partition2)
	require.NoError(t, err)

	// check consensus mana
	require.EqualValues(t, float64(snapshotInfo.PeersAmountsPledged[0]), tests.Mana(t, node1).Consensus)
	t.Logf("node1 (%s): %f", node1.ID().String(), tests.Mana(t, node1).Consensus)
	require.EqualValues(t, float64(snapshotInfo.PeersAmountsPledged[1]), tests.Mana(t, node2).Consensus)
	t.Logf("node2 (%s): %f", node2.ID().String(), tests.Mana(t, node2).Consensus)
	require.EqualValues(t, float64(snapshotInfo.PeersAmountsPledged[2]), tests.Mana(t, node3).Consensus)
	t.Logf("node3 (%s): %f", node3.ID().String(), tests.Mana(t, node3).Consensus)
	require.EqualValues(t, float64(snapshotInfo.PeersAmountsPledged[3]), tests.Mana(t, node4).Consensus)
	t.Logf("node4 (%s): %f", node4.ID().String(), tests.Mana(t, node4).Consensus)

	t.Logf("Sending %d data blocks on minority partition", 150)
	blocksToOrphan := tests.SendDataBlocksWithDelay(t, partition1, 150, delayBetweenDataMessages)
	t.Logf("Sending %d data blocks on majority partition", 50)
	blocksToConfirm := tests.SendDataBlocks(t, partition2, 50)

	// merge partitions
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	// sleep 10 seconds to make sure that TSC threshold is exceeded
	time.Sleep(10 * time.Second)

	t.Logf("Sending %d data messages to make sure that all nodes share the same view", 150)
	tests.SendDataBlocksWithDelay(t, n.Peers(), 150, delayBetweenDataMessages)

	tests.RequireBlocksAvailable(t, n.Peers(), blocksToConfirm, time.Minute, tests.Tick, confirmation.Accepted)
	tests.RequireBlocksOrphaned(t, partition1, blocksToOrphan, time.Minute, tests.Tick)
}
