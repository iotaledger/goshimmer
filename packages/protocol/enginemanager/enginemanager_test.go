package enginemanager_test

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestEngineManager_ForkEngineAtEpoch(t *testing.T) {
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

	etf := NewEngineManagerTestFramework(t, identitiesWeights)

	tf := protocol.NewEngineTestFramework(t, protocol.WithEngine(etf.ActiveEngine.Engine))
	tf.AssertEpochState(0)

	acceptedBlocks := make(map[string]bool)

	epoch1IssuingTime := time.Unix(epoch.GenesisTime, 0)

	// Blocks in epoch 1
	tf.Tangle.CreateBlock("1.A", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.CreateBlock("1.B", models.WithStrongParents(tf.Tangle.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.CreateBlock("1.C", models.WithStrongParents(tf.Tangle.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.CreateBlock("1.D", models.WithStrongParents(tf.Tangle.BlockIDs("1.C")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.IssueBlocks("1.A", "1.B", "1.C", "1.D")
	tf.WaitUntilAllTasksProcessed()

	tf.Acceptance.AssertBlockTracked(4)

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"1.A": true,
		"1.B": true,
		"1.C": false,
		"1.D": false,
	}))

	epoch2IssuingTime := time.Unix(epoch.GenesisTime+epoch.Duration, 0)

	// Block in epoch 2, not accepting anything new.
	tf.Tangle.CreateBlock("2.D", models.WithStrongParents(tf.Tangle.BlockIDs("1.D")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch2IssuingTime))
	tf.Tangle.IssueBlocks("2.D")
	tf.WaitUntilAllTasksProcessed()

	// Block in epoch 11
	tf.Tangle.CreateBlock("11.A", models.WithStrongParents(tf.Tangle.BlockIDs("2.D")), models.WithIssuer(identitiesMap["A"]))
	tf.Tangle.IssueBlocks("11.A")
	tf.WaitUntilAllTasksProcessed()

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"1.C":  true,
		"2.D":  false,
		"11.A": false,
	}))

	require.Equal(t, epoch.IndexFromTime(tf.Tangle.Block("11.A").IssuingTime()), epoch.Index(11))

	// Time hasn't advanced past epoch 1
	require.Equal(t, tf.Engine.Storage.Settings.LatestCommitment().Index(), epoch.Index(0))

	tf.Tangle.CreateBlock("11.B", models.WithStrongParents(tf.Tangle.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]))
	tf.Tangle.CreateBlock("11.C", models.WithStrongParents(tf.Tangle.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]))
	tf.Tangle.IssueBlocks("11.B", "11.C")
	tf.WaitUntilAllTasksProcessed()

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
		return tf.Engine.Storage.Settings.LatestCommitment().Index() == epoch.Index(4)
	}, time.Second, 100*time.Millisecond)

	forkedEngine, err := etf.EngineManager.ForkEngineAtEpoch(tf.Engine.Storage.Settings.LatestCommitment().Index())
	require.NoError(t, err)

	{
		tf2 := protocol.NewEngineTestFramework(t, protocol.WithEngine(forkedEngine.Engine))

		// Settings
		// The ChainID of the new engine corresponds to the target epoch of the imported snapshot.
		require.Equal(t, lo.PanicOnErr(tf.Engine.Storage.Commitments.Load(4)).ID(), tf2.Engine.Storage.Settings.ChainID())
		require.Equal(t, tf.Engine.Storage.Settings.LatestCommitment().InnerModel(), tf2.Engine.Storage.Settings.LatestCommitment().InnerModel())
		require.Equal(t, tf.Engine.Storage.Settings.LatestConfirmedEpoch(), tf2.Engine.Storage.Settings.LatestConfirmedEpoch())
		require.Equal(t, tf.Engine.Storage.Settings.LatestStateMutationEpoch(), tf2.Engine.Storage.Settings.LatestStateMutationEpoch())

		tf2.AssertEpochState(4)

		// Bucketed Storage
		for epochIndex := epoch.Index(0); epochIndex <= 4; epochIndex++ {
			originalCommitment, err := tf.Engine.Storage.Commitments.Load(epochIndex)
			require.NoError(t, err)
			importedCommitment, err := tf2.Engine.Storage.Commitments.Load(epochIndex)
			require.NoError(t, err)

			require.Equal(t, originalCommitment, importedCommitment)

			// Check that StateDiffs have been cleared after snapshot import.
			require.NoError(t, tf2.Engine.LedgerState.StateDiffs.StreamCreatedOutputs(epochIndex, func(*ledger.OutputWithMetadata) error {
				return errors.New("StateDiffs created should be empty after snapshot import")
			}))

			require.NoError(t, tf2.Engine.LedgerState.StateDiffs.StreamSpentOutputs(epochIndex, func(*ledger.OutputWithMetadata) error {
				return errors.New("StateDiffs spent should be empty after snapshot import")
			}))

			// RootBlocks
			require.NoError(t, tf.Engine.Storage.RootBlocks.Stream(epochIndex, func(rootBlock models.BlockID) error {
				has, err := tf2.Engine.Storage.RootBlocks.Has(rootBlock)
				require.NoError(t, err)
				require.True(t, has)

				return nil
			}))
		}

		// LedgerState
		require.Equal(t, tf.Engine.LedgerState.UnspentOutputs.IDs.Size(), tf2.Engine.LedgerState.UnspentOutputs.IDs.Size())
		require.Equal(t, tf.Engine.LedgerState.UnspentOutputs.IDs.Root(), tf2.Engine.LedgerState.UnspentOutputs.IDs.Root())
		require.NoError(t, tf.Engine.LedgerState.UnspentOutputs.IDs.Stream(func(outputID utxo.OutputID) bool {
			require.True(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(outputID))
			return true
		}))

		// SybilProtection
		require.Equal(t, lo.PanicOnErr(tf.Engine.SybilProtection.Weights().Map()), lo.PanicOnErr(tf2.Engine.SybilProtection.Weights().Map()))
		require.Equal(t, tf.Engine.SybilProtection.Weights().TotalWeight(), tf2.Engine.SybilProtection.Weights().TotalWeight())
		require.Equal(t, tf.Engine.SybilProtection.Weights().Root(), tf2.Engine.SybilProtection.Weights().Root())

		// ThroughputQuota
		require.Equal(t, tf.Engine.ThroughputQuota.BalanceByIDs(), tf2.Engine.ThroughputQuota.BalanceByIDs())
		require.Equal(t, tf.Engine.ThroughputQuota.TotalBalance(), tf2.Engine.ThroughputQuota.TotalBalance())

		// Attestations for the targetEpoch only
		require.Equal(t, lo.PanicOnErr(tf.Engine.NotarizationManager.Attestations.Get(4)).Root(), lo.PanicOnErr(tf2.Engine.NotarizationManager.Attestations.Get(4)).Root())
		require.NoError(t, lo.PanicOnErr(tf.Engine.NotarizationManager.Attestations.Get(4)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf2.Engine.NotarizationManager.Attestations.Get(4))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		require.NotEqual(t, tf.Engine.Storage.Directory, tf2.Engine.Storage.Directory)

		require.NoError(t, etf.EngineManager.SetActiveInstance(forkedEngine))

		active, err := etf.EngineManager.LoadActiveEngine()
		require.NoError(t, err)
		require.Equal(t, active.Storage.Directory, forkedEngine.Storage.Directory)
	}
}
