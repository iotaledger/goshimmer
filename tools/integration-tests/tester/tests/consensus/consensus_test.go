package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
)

// TestSimpleDoubleSpend tests whether consensus is able to resolve a simple double spend.
// We spawn a network of 2 nodes containing 40% and 20% of consensus mana respectively,
// let them both issue conflicting transactions, and assert that the transaction
// issued by the 40% node gains a high GoF while the other one gets "none" GoF over time as the 20% consensus mana
// node puts its weight to the 40% issued tx making it reach 60% AW and hence high GoF.
// The genesis seed contains 800000 tokens which we will use to issue conflicting transactions from both nodes.
func TestSimpleDoubleSpend(t *testing.T) {
	const (
		peer1SeedBase58                 = "Bk69VaYsRuiAaKn8hK6KxUj45X5dED3ueRtxfYnsh4Q8" // peerID jnaC6ZyWuw
		peer2SeedBase58                 = "HUH4rmxUxMZBBtHJ4QM5Ts6s8DP3HnFpChejntnCxto2" // peerID iNvPFvkfSDp
		peer1Pledged                    = 800000.0                                       // 40%
		peer2Pledged                    = 400000.0                                       // 20%
		actualGenesisTokenAmount uint64 = 800000                                         // 40%
		numberOfConflictingTxs          = 10
	)

	var (
		expectedCManaNode1AfterTxConf = float64(tests.ConsensusSnapshotDetails.PeersAmountsPledged[0]) + float64(tests.ConsensusSnapshotDetails.GenesisTokenAmount)

		peer1IdentSeed = func() []byte {
			seedBytes, err := base58.Decode(tests.ConsensusSnapshotDetails.PeersSeedBase58[0])
			require.NoError(t, err)
			return seedBytes
		}()

		peer2IdentSeed = func() []byte {
			seedBytes, err := base58.Decode(tests.ConsensusSnapshotDetails.PeersSeedBase58[1])
			require.NoError(t, err)
			return seedBytes
		}()
	)

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetworkNoAutomaticManualPeering(ctx, "test_simple_double_spend", 2,
		framework.CreateNetworkConfig{
			StartSynced: true,
			Faucet:      false,
			Activity:    false,
			Autopeering: false,
		}, func(peerIndex int, cfg config.GoShimmer) config.GoShimmer {
			cfg.MessageLayer.Snapshot.File = tests.ConsensusSnapshotDetails.FilePath
			cfg.UseNodeSeedAsWalletSeed = true
			switch peerIndex {
			case 0:
				cfg.Seed = peer1IdentSeed
			case 1:
				cfg.Seed = peer2IdentSeed
			}
			return cfg
		})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	var (
		node1 = n.Peers()[0]
		node2 = n.Peers()[1]

		genesis1Wallet = createGenesisWallet(node1)
		genesis2Wallet = createGenesisWallet(node2)
	)

	// check consensus mana
	require.EqualValues(t, tests.ConsensusSnapshotDetails.PeersAmountsPledged[0], tests.Mana(t, node1).Consensus)
	require.EqualValues(t, tests.ConsensusSnapshotDetails.PeersAmountsPledged[1], tests.Mana(t, node2).Consensus)

	txs1 := []*ledgerstate.Transaction{}
	txs2 := []*ledgerstate.Transaction{}
	// send transactions on the seperate partitions
	for i := 0; i < numberOfConflictingTxs; i++ {
		t.Logf("issuing conflict %d", i+1)
		// This builds transactions that move the genesis funds on the first partition.
		// Funds move from address 1 -> address 2 -> address 3...
		txs1 = append(txs1, sendConflictingTx(t, genesis1Wallet, genesis1Wallet.Seed().Address(uint64(i+1)), uint64(tests.ConsensusSnapshotDetails.GenesisTokenAmount), node1, gof.Medium))
		t.Logf("issuing other conflict %d", i+1)
		// This builds transactions that move the genesis funds on the second partition
		txs2 = append(txs2, sendConflictingTx(t, genesis2Wallet, genesis2Wallet.Seed().Address(uint64(i+1)), uint64(tests.ConsensusSnapshotDetails.GenesisTokenAmount), node2, gof.Low))
	}

	// merge partitions
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	// conflicting txs should have spawned branches
	require.Eventually(t, func() bool {
		res1, err := node1.GetTransactionMetadata(txs1[0].ID().Base58())
		require.NoError(t, err)
		res2, err := node2.GetTransactionMetadata(txs2[0].ID().Base58())
		require.NoError(t, err)
		return res1.BranchID != ledgerstate.MasterBranchID.String() &&
			res2.BranchID != ledgerstate.MasterBranchID.String()
	}, tests.Timeout, tests.Tick)

	// we issue msgs on both nodes so the txs' GoF can change, given that they are dependent on their
	// attachments' GoF. if msgs would only be issued on node 2 or 1, they weight would never surpass 50%.
	tests.SendDataMessages(t, n.Peers(), 50)

	for i := 0; i < numberOfConflictingTxs; i++ {
		tests.RequireGradeOfFinalityEqual(t, n.Peers(), tests.ExpectedTxsStates{
			txs1[i].ID().Base58(): {
				GradeOfFinality: tests.GoFPointer(gof.High),
				Solid:           tests.True(),
			},
			txs2[i].ID().Base58(): {
				GradeOfFinality: tests.GoFPointer(gof.None),
				Solid:           tests.True(),
			},
		}, time.Minute, tests.Tick)
	}
	require.Eventually(t, func() bool {
		return expectedCManaNode1AfterTxConf == tests.Mana(t, node1).Consensus
	}, tests.Timeout, tests.Tick)
}

func sendConflictingTx(t *testing.T, wallet *wallet.Wallet, targetAddr address.Address, actualGenesisTokenAmount uint64, node *framework.Node, expectedGoF gof.GradeOfFinality) *ledgerstate.Transaction {
	tx, err := wallet.SendFunds(
		sendoptions.Destination(targetAddr, actualGenesisTokenAmount),
		sendoptions.ConsensusManaPledgeID(base58.Encode(node.ID().Bytes())),
		sendoptions.AccessManaPledgeID(base58.Encode(node.ID().Bytes())),
		sendoptions.UsePendingOutputs(true))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		balance := tests.Balance(t, node, targetAddr.Address(), ledgerstate.ColorIOTA)
		return balance == actualGenesisTokenAmount
	}, tests.Timeout, tests.Tick)

	tests.RequireGradeOfFinalityEqual(t, []*framework.Node{node}, tests.ExpectedTxsStates{
		tx.ID().Base58(): {
			GradeOfFinality: tests.GoFPointer(expectedGoF),
			Solid:           tests.True(),
		},
	}, time.Minute, tests.Tick)
	return tx
}

func createGenesisWallet(node *framework.Node) *wallet.Wallet {
	webConn := wallet.GenericConnector(wallet.NewWebConnector(node.BaseURL()))
	return wallet.New(wallet.Import(walletseed.NewSeed(framework.GenesisSeed), 0, []bitmask.BitMask{}, nil), webConn)
}
