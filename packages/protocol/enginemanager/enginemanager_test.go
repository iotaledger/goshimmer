package enginemanager_test

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestEngineManager_ForkEngineAtEpoch(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*10

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

	tf := engine.NewTestFramework(t, workers.CreateGroup("TestFramework"), etf.ActiveEngine.Engine)
	tf.AssertEpochState(0)

	acceptedBlocks := make(map[string]bool)

	epoch1IssuingTime := time.Unix(epoch.GenesisTime, 0)

	// Blocks in epoch 1
	tf.BlockDAG.CreateBlock("1.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.BlockDAG.CreateBlock("1.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.BlockDAG.CreateBlock("1.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.BlockDAG.CreateBlock("1.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.C")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.BlockDAG.IssueBlocks("1.A", "1.B", "1.C", "1.D")

	tf.VirtualVoting.AssertBlockTracked(4)

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"1.A": true,
		"1.B": true,
		"1.C": false,
		"1.D": false,
	}))

	epoch2IssuingTime := time.Unix(epoch.GenesisTime+epoch.Duration, 0)

	// Block in epoch 2, not accepting anything new.
	tf.BlockDAG.CreateBlock("2.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch2IssuingTime))
	tf.BlockDAG.IssueBlocks("2.D")

	// Block in epoch 11
	tf.BlockDAG.CreateBlock("11.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("2.D")), models.WithIssuer(identitiesMap["A"]))
	tf.BlockDAG.IssueBlocks("11.A")

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"1.C":  true,
		"2.D":  false,
		"11.A": false,
	}))

	require.Equal(t, epoch.IndexFromTime(tf.BlockDAG.Block("11.A").IssuingTime()), epoch.Index(11))

	// Time hasn't advanced past epoch 1
	require.Equal(t, tf.Instance.Storage.Settings.LatestCommitment().Index(), epoch.Index(0))

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

	// Time has advanced to epoch 10 because of A.5, rendering 10 - MinimumCommittableAge(6) = 4 epoch committable
	require.Eventually(t, func() bool {
		return tf.Instance.Storage.Settings.LatestCommitment().Index() == epoch.Index(4)
	}, time.Second, 100*time.Millisecond)

	forkedEngine, err := etf.EngineManager.ForkEngineAtEpoch(tf.Instance.Storage.Settings.LatestCommitment().Index())
	require.NoError(t, err)

	{
		tf2 := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework2"), forkedEngine.Engine)

		// Settings
		// The ChainID of the new engine corresponds to the target epoch of the imported snapshot.
		require.Equal(t, lo.PanicOnErr(tf.Instance.Storage.Commitments.Load(4)).ID(), tf2.Instance.Storage.Settings.ChainID())
		require.Equal(t, tf.Instance.Storage.Settings.LatestCommitment(), tf2.Instance.Storage.Settings.LatestCommitment())
		require.Equal(t, tf.Instance.Storage.Settings.LatestConfirmedEpoch(), tf2.Instance.Storage.Settings.LatestConfirmedEpoch())
		require.Equal(t, tf.Instance.Storage.Settings.LatestStateMutationEpoch(), tf2.Instance.Storage.Settings.LatestStateMutationEpoch())

		tf2.AssertEpochState(4)

		// Bucketed Storage
		for epochIndex := epoch.Index(0); epochIndex <= 4; epochIndex++ {
			originalCommitment, err := tf.Instance.Storage.Commitments.Load(epochIndex)
			require.NoError(t, err)
			importedCommitment, err := tf2.Instance.Storage.Commitments.Load(epochIndex)
			require.NoError(t, err)

			require.Equal(t, originalCommitment, importedCommitment)

			// Check that StateDiffs have been cleared after snapshot import.
			require.NoError(t, tf2.Instance.LedgerState.StateDiffs.StreamCreatedOutputs(epochIndex, func(*ledger.OutputWithMetadata) error {
				return errors.New("StateDiffs created should be empty after snapshot import")
			}))

			require.NoError(t, tf2.Instance.LedgerState.StateDiffs.StreamSpentOutputs(epochIndex, func(*ledger.OutputWithMetadata) error {
				return errors.New("StateDiffs spent should be empty after snapshot import")
			}))

			// RootBlocks
			require.NoError(t, tf.Instance.Storage.RootBlocks.Stream(epochIndex, func(rootBlock models.BlockID) error {
				has, err := tf2.Instance.Storage.RootBlocks.Has(rootBlock)
				require.NoError(t, err)
				require.True(t, has)

				return nil
			}))
		}

		// LedgerState
		require.Equal(t, tf.Instance.LedgerState.UnspentOutputs.IDs.Size(), tf2.Instance.LedgerState.UnspentOutputs.IDs.Size())
		require.Equal(t, tf.Instance.LedgerState.UnspentOutputs.IDs.Root(), tf2.Instance.LedgerState.UnspentOutputs.IDs.Root())
		require.NoError(t, tf.Instance.LedgerState.UnspentOutputs.IDs.Stream(func(outputID utxo.OutputID) bool {
			require.True(t, tf2.Instance.LedgerState.UnspentOutputs.IDs.Has(outputID))
			return true
		}))

		// SybilProtection
		require.Equal(t, lo.PanicOnErr(tf.Instance.SybilProtection.Weights().Map()), lo.PanicOnErr(tf2.Instance.SybilProtection.Weights().Map()))
		require.Equal(t, tf.Instance.SybilProtection.Weights().TotalWeight(), tf2.Instance.SybilProtection.Weights().TotalWeight())
		require.Equal(t, tf.Instance.SybilProtection.Weights().Root(), tf2.Instance.SybilProtection.Weights().Root())

		// ThroughputQuota
		require.Equal(t, tf.Instance.ThroughputQuota.BalanceByIDs(), tf2.Instance.ThroughputQuota.BalanceByIDs())
		require.Equal(t, tf.Instance.ThroughputQuota.TotalBalance(), tf2.Instance.ThroughputQuota.TotalBalance())

		// Attestations for the targetEpoch only
		require.Equal(t, lo.PanicOnErr(tf.Instance.NotarizationManager.Attestations.Get(4)).Root(), lo.PanicOnErr(tf2.Instance.NotarizationManager.Attestations.Get(4)).Root())
		require.NoError(t, lo.PanicOnErr(tf.Instance.NotarizationManager.Attestations.Get(4)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf2.Instance.NotarizationManager.Attestations.Get(4))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		require.NotEqual(t, tf.Instance.Storage.Directory, tf2.Instance.Storage.Directory)

		require.NoError(t, etf.EngineManager.SetActiveInstance(forkedEngine))

		active, err := etf.EngineManager.LoadActiveEngine()
		require.NoError(t, err)
		require.Equal(t, active.Storage.Directory, forkedEngine.Storage.Directory)
	}
}
