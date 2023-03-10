package consensus

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/hive.go/ds/bitmask"
	"github.com/iotaledger/hive.go/lo"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestSimpleDoubleSpend tests whether consensus is able to resolve a simple double spend in a partition.
// We set the TSC and MinSlotCommittableAge threshold so that nodes do not commit and blocks are not orphaned due to TSC.
// TODO: update the description to fit the test
// We spawn a network of 2 nodes containing 40% and 20% of consensus mana respectively,
// let them both issue conflicting transactions, and assert that the transaction
// issued by the 40% node will be accepted while the other one gets rejected over time as the 20% consensus mana
// node puts its weight to the 40% issued tx making it reach
// The genesis seed contains 800000 tokens which we will use to issue conflicting transactions from both nodes.
func TestSimpleDoubleSpend(t *testing.T) {
	const (
		numberOfConflictingTxs = 10
	)

	snapshotOptions := tests.ConsensusSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 4,
		framework.CreateNetworkConfig{
			StartSynced: true,
			Faucet:      false,
			Activity:    false,
			Autopeering: false,
			Snapshot:    snapshotOptions,
		}, tests.CommonSnapshotConfigFunc(t, snapshotInfo, func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
			conf.UseNodeSeedAsWalletSeed = true
			conf.ValidatorActivityWindow = 10 * time.Minute
			conf.Protocol.TimeSinceConfirmationThreshold = 10 * time.Minute
			return conf
		}))

	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	const delayBetweenDataBlocks = 100 * time.Millisecond
	dataBlocksAmount := len(n.Peers()) * 10

	var (
		node1 = n.Peers()[0]
		node2 = n.Peers()[1]
		node3 = n.Peers()[2]
		node4 = n.Peers()[3]

		genesis1Wallet = createGenesisWallet(node1)
		genesis2Wallet = createGenesisWallet(node2)
	)

	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	// issue blocks on all nodes
	tests.SendDataBlocks(t, n.Peers(), 50)

	// split partitions
	err = n.CreatePartitionsManualPeering(ctx, []*framework.Node{node1}, []*framework.Node{node2}, []*framework.Node{node3, node4})
	require.NoError(t, err)

	// check consensus mana
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[0], tests.Mana(t, node1).Consensus)
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[1], tests.Mana(t, node2).Consensus)
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[2], tests.Mana(t, node3).Consensus)
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[3], tests.Mana(t, node4).Consensus)

	var txs1 []*devnetvm.Transaction
	var txs2 []*devnetvm.Transaction
	// send transactions on the separate partitions
	for i := 0; i < numberOfConflictingTxs; i++ {
		t.Logf("issuing conflict %d", i+1)
		// This builds transactions that move the genesis funds on the first partition.
		// Funds move from address 1 -> address 2 -> address 3...
		txs1 = append(txs1, sendConflictingTx(t, genesis1Wallet, genesis1Wallet.Seed().Address(uint64(i+1)), snapshotInfo.GenesisTokenAmount, node1))
		t.Logf("issuing other conflict %d", i+1)
		// This builds transactions that move the genesis funds on the second partition
		txs2 = append(txs2, sendConflictingTx(t, genesis2Wallet, genesis2Wallet.Seed().Address(uint64(i+1)), snapshotInfo.GenesisTokenAmount, node2))
	}

	// merge partitions
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	t.Logf("Sending %d data blocks to make ConfirmationState converge", dataBlocksAmount)
	tests.SendDataBlocksWithDelay(t, n.Peers(), dataBlocksAmount, delayBetweenDataBlocks)
	t.Logf("Sending %d data blocks to make ConfirmationState converge... done", dataBlocksAmount)

	t.Logf("Waiting for conflicting transactions to be marked...")
	// conflicting txs should have spawned conflicts
	require.Eventually(t, func() bool {
		res1, err := node1.GetTransactionMetadata(txs1[0].ID().Base58())
		require.NoError(t, err)
		res2, err := node2.GetTransactionMetadata(txs2[0].ID().Base58())
		require.NoError(t, err)
		return len(res1.ConflictIDs) > 0 && len(res2.ConflictIDs) > 0
	}, tests.Timeout, tests.Tick)
	t.Logf("Waiting for conflicting transactions to be marked... done")

	t.Logf("Sending data blocks to resolve the conflict...")
	// we issue blks on both nodes so the txs' ConfirmationState can change, given that they are dependent on their
	// attachments' ConfirmationState. if blks would only be issued on node 2 or 1, they weight would never surpass 50%.
	tests.SendDataBlocks(t, n.Peers(), 50)
	t.Logf("Sending data blocks to resolve the conflict... done")

	t.Logf("Making sure that conflicts are resolved...")
	for i := 0; i < numberOfConflictingTxs; i++ {
		tests.RequireConfirmationStateEqual(t, n.Peers(), tests.ExpectedTxsStates{
			txs1[i].ID().Base58(): {
				ConfirmationState: confirmation.Accepted,
				Solid:             tests.True(),
			},
			txs2[i].ID().Base58(): {
				ConfirmationState: confirmation.Rejected,
				Solid:             tests.True(),
			},
		}, time.Minute, tests.Tick)
	}
	t.Logf("Making sure that conflicts are resolved... done")

}

func sendConflictingTx(t *testing.T, wallet *wallet.Wallet, targetAddr address.Address, actualGenesisTokenAmount uint64, node *framework.Node) *devnetvm.Transaction {
	tx, err := wallet.SendFunds(
		sendoptions.Destination(targetAddr, actualGenesisTokenAmount),
		sendoptions.ConsensusManaPledgeID(base58.Encode(lo.PanicOnErr(node.ID().Bytes()))),
		sendoptions.AccessManaPledgeID(base58.Encode(lo.PanicOnErr(node.ID().Bytes()))),
		sendoptions.UsePendingOutputs(true))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		balance := tests.Balance(t, node, targetAddr.Address(), devnetvm.ColorIOTA)
		return balance == actualGenesisTokenAmount
	}, tests.Timeout, tests.Tick)

	return tx
}

func createGenesisWallet(node *framework.Node) *wallet.Wallet {
	webConn := wallet.GenericConnector(wallet.NewWebConnector(node.BaseURL()))
	return wallet.New(wallet.Import(walletseed.NewSeed(framework.GenesisSeedBytes), 0, []bitmask.BitMask{}, nil), webConn)
}
