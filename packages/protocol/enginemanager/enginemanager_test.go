package enginemanager_test

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func TestEngineManager_ForkEngineAtSlot(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())

	identitiesMap := map[string]ed25519.PublicKey{
		"A": identity.GenerateIdentity().PublicKey(),
		"B": identity.GenerateIdentity().PublicKey(),
		"C": identity.GenerateIdentity().PublicKey(),
		"D": identity.GenerateIdentity().PublicKey(),
	}

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.New(identitiesMap["A"]).PublicKey(): 25,
		identity.New(identitiesMap["B"]).PublicKey(): 25,
		identity.New(identitiesMap["C"]).PublicKey(): 25,
		identity.New(identitiesMap["D"]).PublicKey(): 25,
	}

	etf := NewEngineManagerTestFramework(t, workers.CreateGroup("EngineManagerTestFramework"), identitiesWeights)

	tf := engine.NewTestFramework(t, workers.CreateGroup("TestFramework"), etf.ActiveEngine)
	tf.AssertSlotState(0)

	acceptedBlocks := make(map[string]bool)

	slot1IssuingTime := tf.SlotTimeProvider().StartTime(1)

	// Blocks in slot 1
	tf.BlockDAG.CreateBlock("1.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot1IssuingTime))
	tf.BlockDAG.CreateBlock("1.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot1IssuingTime))
	tf.BlockDAG.CreateBlock("1.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(slot1IssuingTime))
	tf.BlockDAG.CreateBlock("1.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.C")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(slot1IssuingTime))
	tf.BlockDAG.IssueBlocks("1.A", "1.B", "1.C", "1.D")

	tf.Booker.AssertBlockTracked(4)

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"1.A": true,
		"1.B": true,
		"1.C": false,
		"1.D": false,
	}))

	slot2IssuingTime := tf.SlotTimeProvider().StartTime(2)

	// Block in slot 2, not accepting anything new.
	tf.BlockDAG.CreateBlock("2.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(slot2IssuingTime))
	tf.BlockDAG.IssueBlocks("2.D")

	// Block in slot 11
	tf.BlockDAG.CreateBlock("11.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("2.D")), models.WithIssuer(identitiesMap["A"]))
	tf.BlockDAG.IssueBlocks("11.A")

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"1.C":  true,
		"2.D":  false,
		"11.A": false,
	}))

	require.Equal(t, tf.SlotTimeProvider().IndexFromTime(tf.BlockDAG.Block("11.A").IssuingTime()), slot.Index(11))

	// Time hasn't advanced past slot 1
	require.Equal(t, tf.Instance.Storage.Settings.LatestCommitment().Index(), slot.Index(0))

	tf.BlockDAG.CreateBlock("11.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]))
	tf.BlockDAG.CreateBlock("11.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]))
	tf.BlockDAG.IssueBlocks("11.B", "11.C")

	// Some blocks got evicted, and we have to restart evaluating with a new map
	acceptedBlocks = make(map[string]bool)
	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"2.D":  true,
		"11.A": true,
		"11.B": false,
		"11.C": false,
	}))

	// Time has advanced to slot 10 because of A.5, rendering 10 - MinimumCommittableAge(6) = 4 slot committable
	require.Eventually(t, func() bool {
		return tf.Instance.Storage.Settings.LatestCommitment().Index() == slot.Index(4)
	}, time.Second, 100*time.Millisecond)

	forkedEngine, err := etf.EngineManager.ForkEngineAtSlot(tf.Instance.Storage.Settings.LatestCommitment().Index())
	require.NoError(t, err)

	{
		tf2 := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework2"), forkedEngine)

		// Settings
		// The ChainID of the new engine should correspond to genesis.
		require.Equal(t, lo.PanicOnErr(tf.Instance.Storage.Commitments.Load(0)).ID(), tf2.Instance.Storage.Settings.ChainID())
		require.Equal(t, lo.PanicOnErr(tf.Instance.Storage.Commitments.Load(4)).ID(), lo.PanicOnErr(tf2.Instance.Storage.Commitments.Load(4)).ID())
		require.Equal(t, lo.PanicOnErr(tf.Instance.Notarization.Attestations().Get(4)).Root(), lo.PanicOnErr(tf2.Instance.Notarization.Attestations().Get(4)).Root())
		// We cache bytes here by getting ID so the next Equal doesn't fail.
		require.Equal(t, tf.Instance.Storage.Settings.LatestCommitment().ID(), tf2.Instance.Storage.Settings.LatestCommitment().ID())
		require.Equal(t, tf.Instance.Storage.Settings.LatestCommitment(), tf2.Instance.Storage.Settings.LatestCommitment())
		require.Equal(t, tf.Instance.Storage.Settings.LatestConfirmedSlot(), tf2.Instance.Storage.Settings.LatestConfirmedSlot())
		require.Equal(t, tf.Instance.Storage.Settings.LatestStateMutationSlot(), tf2.Instance.Storage.Settings.LatestStateMutationSlot())

		tf2.AssertSlotState(4)

		// Bucketed Storage
		for slotIndex := slot.Index(0); slotIndex <= 4; slotIndex++ {
			originalCommitment, err := tf.Instance.Storage.Commitments.Load(slotIndex)
			require.NoError(t, err)
			importedCommitment, err := tf2.Instance.Storage.Commitments.Load(slotIndex)
			require.NoError(t, err)

			require.Equal(t, originalCommitment, importedCommitment)

			// Check that StateDiffs have been cleared after snapshot import.
			require.NoError(t, tf2.Instance.Ledger.StateDiffs().StreamCreatedOutputs(slotIndex, func(*mempool.OutputWithMetadata) error {
				return errors.New("StateDiffs created should be empty after snapshot import")
			}))

			require.NoError(t, tf2.Instance.Ledger.StateDiffs().StreamSpentOutputs(slotIndex, func(*mempool.OutputWithMetadata) error {
				return errors.New("StateDiffs spent should be empty after snapshot import")
			}))

			// RootBlocks
			require.NoError(t, tf.Instance.Storage.RootBlocks.Stream(slotIndex, func(rootBlock models.BlockID, _ commitment.ID) error {
				has, err := tf2.Instance.Storage.RootBlocks.Has(rootBlock)
				require.NoError(t, err)
				require.True(t, has)

				return nil
			}))
		}

		// LedgerState
		require.Equal(t, tf.Instance.Ledger.UnspentOutputs().IDs().Size(), tf2.Instance.Ledger.UnspentOutputs().IDs().Size())
		require.Equal(t, tf.Instance.Ledger.UnspentOutputs().IDs().Root(), tf2.Instance.Ledger.UnspentOutputs().IDs().Root())
		require.NoError(t, tf.Instance.Ledger.UnspentOutputs().IDs().Stream(func(outputID utxo.OutputID) bool {
			require.True(t, tf2.Instance.Ledger.UnspentOutputs().IDs().Has(outputID))
			return true
		}))

		// SybilProtection
		require.Equal(t, lo.PanicOnErr(tf.Instance.SybilProtection.Weights().Map()), lo.PanicOnErr(tf2.Instance.SybilProtection.Weights().Map()))
		require.Equal(t, tf.Instance.SybilProtection.Weights().TotalWeight(), tf2.Instance.SybilProtection.Weights().TotalWeight())
		require.Equal(t, tf.Instance.SybilProtection.Weights().Root(), tf2.Instance.SybilProtection.Weights().Root())

		// ThroughputQuota
		require.Equal(t, tf.Instance.ThroughputQuota.BalanceByIDs(), tf2.Instance.ThroughputQuota.BalanceByIDs())
		require.Equal(t, tf.Instance.ThroughputQuota.TotalBalance(), tf2.Instance.ThroughputQuota.TotalBalance())

		// Attestations for the targetSlot only
		require.Equal(t, lo.PanicOnErr(tf.Instance.Notarization.Attestations().Get(4)).Root(), lo.PanicOnErr(tf2.Instance.Notarization.Attestations().Get(4)).Root())
		require.NoError(t, lo.PanicOnErr(tf.Instance.Notarization.Attestations().Get(4)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf2.Instance.Notarization.Attestations().Get(4))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		require.NotEqual(t, tf.Instance.Storage.Directory, tf2.Instance.Storage.Directory)

		require.NoError(t, etf.EngineManager.SetActiveInstance(forkedEngine))

		forkedEngine.Shutdown()

		active, err := etf.EngineManager.LoadActiveEngine()
		require.NoError(t, err)
		require.Equal(t, active.Storage.Directory, forkedEngine.Storage.Directory)
	}
}
