package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/bitmask"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestSimpleDoubleSpend tests whether consensus is able to resolve a simple double spend.
// We spawn a network of 2 nodes containing 40% and 20% of consensus mana respectively,
// let them both issue conflicting transactions, and assert that the transaction
// issued by the 40% node gains a high ConfirmationState while the other one gets "none" ConfirmationState over time as the 20% consensus mana
// node puts its weight to the 40% issued tx making it reach 60% AW and hence high ConfirmationState.
// The genesis seed contains 800000 tokens which we will use to issue conflicting transactions from both nodes.
func TestSimpleDoubleSpend(t *testing.T) {
	const (
		numberOfConflictingTxs = 10
	)

	snapshotInfo := tests.ConsensusSnapshotDetails

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetworkNoAutomaticManualPeering(ctx, "test_simple_double_spend", 2,
		framework.CreateNetworkConfig{
			StartSynced: true,
			Faucet:      false,
			Activity:    false,
			Autopeering: false,
			PeerMaster:  false,
			Snapshot:    snapshotInfo,
		}, tests.CommonSnapshotConfigFunc(t, snapshotInfo, func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
			conf.UseNodeSeedAsWalletSeed = true
			return conf
		}))

	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	const delayBetweenDataBlocks = 100 * time.Millisecond
	dataBlocksAmount := len(n.Peers()) * 3

	var (
		node1 = n.Peers()[0]
		node2 = n.Peers()[1]

		genesis1Wallet = createGenesisWallet(node1)
		genesis2Wallet = createGenesisWallet(node2)
	)

	// merge partitions
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	tests.SendDataBlocks(t, n.Peers(), 50)

	// split partitions
	err = n.CreatePartitionsManualPeering(ctx, []*framework.Node{node1}, []*framework.Node{node2})
	require.NoError(t, err)

	// check consensus mana
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[0], tests.Mana(t, node1).Consensus)
	require.EqualValues(t, snapshotInfo.PeersAmountsPledged[1], tests.Mana(t, node2).Consensus)

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

	// conflicting txs should have spawned conflicts
	require.Eventually(t, func() bool {
		res1, err := node1.GetTransactionMetadata(txs1[0].ID().Base58())
		require.NoError(t, err)
		res2, err := node2.GetTransactionMetadata(txs2[0].ID().Base58())
		require.NoError(t, err)
		return len(res1.ConflictIDs) > 0 && len(res2.ConflictIDs) > 0
	}, tests.Timeout*2, tests.Tick)

	// we issue blks on both nodes so the txs' ConfirmationState can change, given that they are dependent on their
	// attachments' ConfirmationState. if blks would only be issued on node 2 or 1, they weight would never surpass 50%.
	tests.SendDataBlocks(t, n.Peers(), 50)

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
		}, time.Minute*2, tests.Tick)
	}
}

func sendConflictingTx(t *testing.T, wallet *wallet.Wallet, targetAddr address.Address, actualGenesisTokenAmount uint64, node *framework.Node) *devnetvm.Transaction {
	tx, err := wallet.SendFunds(
		sendoptions.Destination(targetAddr, actualGenesisTokenAmount),
		sendoptions.ConsensusManaPledgeID(base58.Encode(node.ID().Bytes())),
		sendoptions.AccessManaPledgeID(base58.Encode(node.ID().Bytes())),
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
