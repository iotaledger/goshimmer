package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestSimpleDoubleSpend tests whether consensus is able to resolve a simple double spend.
// We spawn a network of 2 nodes containing 40% and 20% of consensus mana respectively,
// let them both issue conflicting transactions, and assert that the transaction
// issued by the 40% node gains a high GoF while the other one gets "none" GoF over time as the 20% consensus mana
// node puts its weight to the 40% issued tx making it reach 60% AW and hence high GoF.
// The genesis seed contains 800000 tokens which we will use to issue conflicting transactions from both nodes.
func TestSimpleDoubleSpend(t *testing.T) {
	const (
		peer1SeedBase58                      = "Bk69VaYsRuiAaKn8hK6KxUj45X5dED3ueRtxfYnsh4Q8" // peerID jnaC6ZyWuw
		peer2SeedBase58                      = "HUH4rmxUxMZBBtHJ4QM5Ts6s8DP3HnFpChejntnCxto2" // peerID iNvPFvkfSDp
		peer1Pledged                         = 800000.0                                       // 40%
		peer2Pledged                         = 400000.0                                       // 20%
		actualGenesisTokenAmount      uint64 = 800000                                         // 40%
		expectedCManaNode1AfterTxConf        = peer1Pledged + float64(actualGenesisTokenAmount)
	)

	var (
		peer1IdentSeed = func() []byte {
			seedBytes, err := base58.Decode(peer1SeedBase58)
			require.NoError(t, err)
			return seedBytes
		}()

		peer2IdentSeed = func() []byte {
			seedBytes, err := base58.Decode(peer2SeedBase58)
			require.NoError(t, err)
			return seedBytes
		}()
	)

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 2, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      false,
		Activity:    false,
	}, func(peerIndex int, cfg config.GoShimmer) config.GoShimmer {
		cfg.MessageLayer.Snapshot.File = "/assets/consensus_intgr_snapshot.bin"
		cfg.UseNodeSeedAsWalletSeed = true
		switch peerIndex {
		case 0:
			cfg.Autopeering.Seed = "base58:" + peer1SeedBase58
			cfg.Seed = peer1IdentSeed
		case 1:
			cfg.Autopeering.Seed = "base58:" + peer2SeedBase58
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

		// 1G4a9TMsLE59BUbe76dacsFG63Ue2KN14orS5g4rPeNiK
		node1TargetAddr = node1.Seed.Address(1)
		// 1B64FizyBfUuycSKmAFADRiZo22SJj5pWrXoYvfY78q7L
		node2TargetAddr = node2.Seed.Address(1)
	)

	// check consensus mana
	require.EqualValues(t, peer1Pledged, tests.Mana(t, node1).Consensus)
	require.EqualValues(t, peer2Pledged, tests.Mana(t, node2).Consensus)

	// send conflicting txs on both
	tx1 := sendConflictingTx(t, genesis1Wallet, node1TargetAddr, actualGenesisTokenAmount, node1, gof.Medium)
	tx2 := sendConflictingTx(t, genesis2Wallet, node2TargetAddr, actualGenesisTokenAmount, node2, gof.Low)

	// conflicting txs should have spawned branches
	require.Eventually(t, func() bool {
		res1, err := node1.GetTransactionMetadata(tx1.ID().Base58())
		require.NoError(t, err)
		res2, err := node2.GetTransactionMetadata(tx2.ID().Base58())
		require.NoError(t, err)
		return res1.BranchID != ledgerstate.MasterBranchID.String() &&
			res2.BranchID != ledgerstate.MasterBranchID.String()
	}, tests.Timeout, tests.Tick)

	// we issue Messages on both nodes so the txs' GoF can change, given that they are dependent on their
	// attachments' GoF. if Messages would only be issued on node 2 or 1, they weight would never surpass 50%.
	tests.SendDataMessages(t, n.Peers(), 50)

	tests.RequireGradeOfFinalityEqual(t, n.Peers(), tests.ExpectedTxsStates{
		tx1.ID().Base58(): {
			GradeOfFinality: tests.GoFPointer(gof.High),
			SolidityType:    tests.Solid(),
		},
		tx2.ID().Base58(): {
			GradeOfFinality: tests.GoFPointer(gof.None),
			SolidityType:    tests.Solid(),
		},
	}, time.Minute, tests.Tick)

	require.Eventually(t, func() bool {
		return expectedCManaNode1AfterTxConf == tests.Mana(t, node1).Consensus
	}, tests.Timeout, tests.Tick)
}

func sendConflictingTx(t *testing.T, wallet *wallet.Wallet, targetAddr address.Address, actualGenesisTokenAmount uint64, node *framework.Node, expectedGoF gof.GradeOfFinality) *ledgerstate.Transaction {
	tx, err := wallet.SendFunds(
		sendoptions.Destination(targetAddr, actualGenesisTokenAmount),
		sendoptions.ConsensusManaPledgeID(base58.Encode(node.ID().Bytes())),
		sendoptions.AccessManaPledgeID(base58.Encode(node.ID().Bytes())),
	)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		balance := tests.Balance(t, node, targetAddr.Address(), ledgerstate.ColorIOTA)
		return balance == actualGenesisTokenAmount
	}, tests.Timeout, tests.Tick)

	tests.RequireGradeOfFinalityEqual(t, []*framework.Node{node}, tests.ExpectedTxsStates{
		tx.ID().Base58(): {
			GradeOfFinality: tests.GoFPointer(expectedGoF),
			SolidityType:    tests.Solid(),
		},
	}, time.Minute, tests.Tick)
	return tx
}

func createGenesisWallet(node *framework.Node) *wallet.Wallet {
	webConn := wallet.GenericConnector(wallet.NewWebConnector(node.BaseURL()))
	return wallet.New(wallet.Import(walletseed.NewSeed(framework.GenesisSeed), 0, []bitmask.BitMask{}, nil), webConn)
}
