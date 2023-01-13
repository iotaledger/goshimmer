package protocol

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
)

func TestProtocol(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	testNetwork := network.NewMockedNetwork()

	endpoint1 := testNetwork.Join(identity.GenerateIdentity().ID())
	tempDir := utils.NewDirectory(t.TempDir())

	identitiesWeights := map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	}

	snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir.Path("snapshot.bin"), 100, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights))

	protocol1 := New(endpoint1, WithBaseDirectory(tempDir.Path()), WithSnapshotPath(tempDir.Path("snapshot.bin")))
	protocol1.Run()

	t.Cleanup(func() {
		protocol1.Shutdown()
		protocol1.CongestionControl.WorkerPool().ShutdownComplete.Wait()
		for _, pool := range protocol1.Engine().WorkerPools() {
			pool.ShutdownComplete.Wait()
		}
	})

	commitments := make(map[string]*commitment.Commitment)
	commitments["0"] = commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	commitments["1"] = commitment.New(1, commitments["0"].ID(), types.Identifier{1}, 0)
	commitments["2"] = commitment.New(2, commitments["1"].ID(), types.Identifier{2}, 0)
	commitments["3"] = commitment.New(3, commitments["2"].ID(), types.Identifier{3}, 0)

	protocol1.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["1"],
		Source:     identity.ID{},
	})

	protocol1.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["2"],
		Source:     identity.ID{},
	})

	protocol1.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Source:     identity.ID{},
	})

	endpoint2 := testNetwork.Join(identity.GenerateIdentity().ID())
	tempDir2 := utils.NewDirectory(t.TempDir())

	snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir2.Path("snapshot.bin"), 100, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights))

	protocol2 := New(endpoint2, WithBaseDirectory(tempDir2.Path()), WithSnapshotPath(tempDir2.Path("snapshot.bin")))
	protocol2.Run()

	protocol2.chainManager.Events.CommitmentMissing.Hook(event.NewClosure(func(id commitment.ID) {
		fmt.Println("MISSING", id)
	}))
	protocol2.chainManager.Events.MissingCommitmentReceived.Hook(event.NewClosure(func(id commitment.ID) {
		fmt.Println("MISSING RECEIVED", id)
	}))

	protocol2.networkProtocol.Events.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Source:     identity.ID{},
	})

	tf1 := NewEngineTestFramework(t, WithEngine(protocol1.Engine()))
	tf2 := NewEngineTestFramework(t, WithEngine(protocol2.Engine()))

	tf1.Tangle.CreateBlock("A", models.WithStrongParents(tf1.Tangle.BlockIDs("Genesis")))
	tf1.Tangle.IssueBlocks("A")

	tf1.WaitUntilAllTasksProcessed()
	tf2.WaitUntilAllTasksProcessed()
}

func TestEngine_NonEmptyInitialValidators(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	epoch.GenesisTime = time.Now().Unix()

	tf := NewEngineTestFramework(t)
	tempDir := utils.NewDirectory(t.TempDir())

	identitiesMap := map[string]ed25519.PublicKey{
		"A": identity.GenerateIdentity().PublicKey(),
		"B": identity.GenerateIdentity().PublicKey(),
		"C": identity.GenerateIdentity().PublicKey(),
		"D": identity.GenerateIdentity().PublicKey(),
	}

	identitiesWeights := map[identity.ID]uint64{
		identity.New(identitiesMap["A"]).ID(): 30,
		identity.New(identitiesMap["B"]).ID(): 30,
		identity.New(identitiesMap["C"]).ID(): 30,
		identity.New(identitiesMap["D"]).ID(): 10,
	}

	snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights))

	require.NoError(t, tf.Engine.Initialize(tempDir.Path("genesis_snapshot.bin")))

	tf.Tangle.CreateBlock("1.A", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["A"]))
	tf.Tangle.IssueBlocks("1.A")
	tf.WaitUntilAllTasksProcessed()

	// If the list of validators would be empty, this block will be accepted right away.
	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"1.A": false,
	})

	tf.Tangle.CreateBlock("1.B", models.WithStrongParents(tf.Tangle.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]))
	tf.Tangle.IssueBlocks("1.B")
	tf.WaitUntilAllTasksProcessed()

	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"1.A": false,
		"1.B": false,
	})

	tf.Tangle.CreateBlock("1.C", models.WithStrongParents(tf.Tangle.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]))
	tf.Tangle.IssueBlocks("1.C")
	tf.WaitUntilAllTasksProcessed()

	// ...but it get accepted only when 67% of the active weight is reached.
	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"1.A": true,
		"1.B": false,
		"1.C": false,
	})

	tf.WaitUntilAllTasksProcessed()
}

func TestEngine_BlocksForwardAndRollback(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*10

	fmt.Println("> GenesisTime", time.Unix(epoch.GenesisTime, 0))

	tf := NewEngineTestFramework(t)
	tempDir := utils.NewDirectory(t.TempDir())

	identitiesMap := map[string]ed25519.PublicKey{
		"A": identity.GenerateIdentity().PublicKey(),
		"B": identity.GenerateIdentity().PublicKey(),
		"C": identity.GenerateIdentity().PublicKey(),
		"D": identity.GenerateIdentity().PublicKey(),
	}

	identitiesWeights := map[identity.ID]uint64{
		identity.New(identitiesMap["A"]).ID(): 25,
		identity.New(identitiesMap["B"]).ID(): 25,
		identity.New(identitiesMap["C"]).ID(): 25,
		identity.New(identitiesMap["D"]).ID(): 25,
	}

	snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights))

	require.NoError(t, tf.Engine.Initialize(tempDir.Path("genesis_snapshot.bin")))

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
	require.Equal(t, tf.Engine.Storage.Settings.LatestCommitment().Index(), epoch.Index(4))

	// Dump snapshot for latest committable epoch 4 and check engine equivalence
	{
		require.NoError(t, tf.Engine.WriteSnapshot(tempDir.Path("snapshot_epoch4.bin")))

		tf2 := NewEngineTestFramework(t)

		require.NoError(t, tf2.Engine.Initialize(tempDir.Path("snapshot_epoch4.bin")))

		// Settings
		// The ChainID of the new engine corresponds to the target epoch of the imported snapshot.
		require.Equal(t, lo.PanicOnErr(tf.Engine.Storage.Commitments.Load(4)).ID(), tf2.Engine.Storage.Settings.ChainID())
		require.Equal(t, tf.Engine.Storage.Settings.LatestCommitment(), tf2.Engine.Storage.Settings.LatestCommitment())
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
	}

	// Dump snapshot for epoch 1 and check attestations equivalence
	{
		require.NoError(t, tf.Engine.WriteSnapshot(tempDir.Path("snapshot_epoch1.bin"), 1))

		tf2 := NewEngineTestFramework(t)

		require.NoError(t, tf2.Engine.Initialize(tempDir.Path("snapshot_epoch1.bin")))

		require.Equal(t, epoch.Index(4), tf.Engine.Storage.Settings.LatestCommitment().Index())

		tf2.AssertEpochState(1)

		// Check that we only have attestations for epoch 1.
		require.Equal(t, lo.PanicOnErr(tf.Engine.NotarizationManager.Attestations.Get(1)).Root(), lo.PanicOnErr(tf2.Engine.NotarizationManager.Attestations.Get(1)).Root())
		require.Error(t, lo.Return2(tf2.Engine.NotarizationManager.Attestations.Get(2)))
		require.Error(t, lo.Return2(tf2.Engine.NotarizationManager.Attestations.Get(3)))
		require.Error(t, lo.Return2(tf2.Engine.NotarizationManager.Attestations.Get(4)))
		require.NoError(t, lo.PanicOnErr(tf.Engine.NotarizationManager.Attestations.Get(1)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf2.Engine.NotarizationManager.Attestations.Get(1))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		// RootBlocks
		for epochIndex := epoch.Index(0); epochIndex <= 1; epochIndex++ {
			require.NoError(t, tf.Engine.Storage.RootBlocks.Stream(epochIndex, func(rootBlock models.BlockID) error {
				has, err := tf2.Engine.Storage.RootBlocks.Has(rootBlock)
				require.NoError(t, err)
				require.True(t, has)

				return nil
			}))
		}

		// Block in epoch 2, not accepting anything new.
		tf2.Tangle.CreateBlock("2.D", models.WithStrongParents(tf.Tangle.BlockIDs("1.D")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch2IssuingTime))
		tf2.Tangle.IssueBlocks("2.D")
		tf2.WaitUntilAllTasksProcessed()

		// Block in epoch 11
		tf2.Tangle.CreateBlock("11.A", models.WithStrongParents(tf2.Tangle.BlockIDs("2.D")), models.WithIssuer(identitiesMap["A"]))
		tf2.Tangle.IssueBlocks("11.A")
		tf2.WaitUntilAllTasksProcessed()

		tf2.Tangle.CreateBlock("11.B", models.WithStrongParents(tf2.Tangle.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]))
		tf2.Tangle.CreateBlock("11.C", models.WithStrongParents(tf2.Tangle.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]))
		tf2.Tangle.IssueBlocks("11.B", "11.C")
		tf2.WaitUntilAllTasksProcessed()

		require.Equal(t, epoch.Index(4), tf2.Engine.Storage.Settings.LatestCommitment().Index())

		// Some blocks got evicted, and we have to restart evaluating with a new map
		acceptedBlocks = make(map[string]bool)
		tf2.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
			"2.D":  true,
			"11.A": true,
			"11.B": false,
			"11.C": false,
		}))

		tf2.WaitUntilAllTasksProcessed()
	}

	// Dump snapshot for epoch 2 and check equivalence.
	{
		require.NoError(t, tf.Engine.WriteSnapshot(tempDir.Path("snapshot_epoch2.bin"), 2))

		tf2 := NewEngineTestFramework(t)

		require.NoError(t, tf2.Engine.Initialize(tempDir.Path("snapshot_epoch2.bin")))

		require.Equal(t, epoch.Index(2), tf2.Engine.Storage.Settings.LatestCommitment().Index())

		tf2.AssertEpochState(2)

		// Check that we only have attestations for epoch 2.
		require.Nil(t, lo.Return2(tf2.Engine.NotarizationManager.Attestations.Get(1)))
		require.NoError(t, lo.Return2(tf2.Engine.NotarizationManager.Attestations.Get(2)))
		require.Error(t, lo.Return2(tf2.Engine.NotarizationManager.Attestations.Get(3)))
		require.Error(t, lo.Return2(tf2.Engine.NotarizationManager.Attestations.Get(4)))
		require.NoError(t, lo.PanicOnErr(tf.Engine.NotarizationManager.Attestations.Get(2)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf2.Engine.NotarizationManager.Attestations.Get(2))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		// RootBlocks
		for epochIndex := epoch.Index(0); epochIndex <= 2; epochIndex++ {
			require.NoError(t, tf.Engine.Storage.RootBlocks.Stream(epochIndex, func(rootBlock models.BlockID) error {
				has, err := tf2.Engine.Storage.RootBlocks.Has(rootBlock)
				require.NoError(t, err)
				require.True(t, has)

				return nil
			}))
		}

		tf2.WaitUntilAllTasksProcessed()
	}
}

func TestEngine_TransactionsForwardAndRollback(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*15

	storageDir := t.TempDir()
	storageInstance := storage.New(storageDir, DatabaseVersion, database.WithDBProvider(database.NewDB))

	tf := NewEngineTestFramework(t, WithStorage(storageInstance), WithTangleOptions(
		tangle.WithBookerOptions(
			booker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
			),
		),
	))
	tempDir := utils.NewDirectory(t.TempDir())

	tf.Engine.NotarizationManager.Events.Error.Attach(event.NewClosure(func(err error) {
		panic(err)
	}))

	identitiesMap := map[string]ed25519.PublicKey{
		"A": identity.GenerateIdentity().PublicKey(),
		"B": identity.GenerateIdentity().PublicKey(),
		"C": identity.GenerateIdentity().PublicKey(),
		"D": identity.GenerateIdentity().PublicKey(),
		"Z": identity.GenerateIdentity().PublicKey(),
	}

	identitiesWeights := map[identity.ID]uint64{
		identity.New(identitiesMap["A"]).ID(): 25,
		identity.New(identitiesMap["B"]).ID(): 25,
		identity.New(identitiesMap["C"]).ID(): 25,
		identity.New(identitiesMap["D"]).ID(): 25,
		identity.New(identitiesMap["Z"]).ID(): 0,
	}

	snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights))

	require.NoError(t, tf.Engine.Initialize(tempDir.Path("genesis_snapshot.bin")))

	require.Equal(t, int64(100), tf.Engine.SybilProtection.Validators().TotalWeight())

	acceptedBlocks := make(map[string]bool)
	epoch1IssuingTime := time.Unix(epoch.GenesisTime, 0)

	{
		tf.Tangle.CreateBlock("1.Z", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithPayload(tf.Tangle.CreateTransaction("Tx1", 2, "Genesis")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.Tangle.CreateBlock("1.Z*", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithPayload(tf.Tangle.CreateTransaction("Tx1*", 2, "Genesis")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.Tangle.CreateBlock("1.A", models.WithStrongParents(tf.Tangle.BlockIDs("1.Z")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.Tangle.CreateBlock("1.B", models.WithStrongParents(tf.Tangle.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.Tangle.CreateBlock("1.C", models.WithStrongParents(tf.Tangle.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.Tangle.CreateBlock("1.D", models.WithStrongParents(tf.Tangle.BlockIDs("1.C")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.Tangle.IssueBlocks("1.Z", "1.Z*", "1.A", "1.B", "1.C", "1.D")
		tf.WaitUntilAllTasksProcessed()

		tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
			"1.Z":  true,
			"1.Z*": false,
			"1.A":  true,
			"1.B":  true,
			"1.C":  false,
			"1.D":  false,
		}))

		acceptedConflicts := make(map[string]confirmation.State)
		tf.Acceptance.ValidateConflictAcceptance(lo.MergeMaps(acceptedConflicts, map[string]confirmation.State{
			"Tx1":  confirmation.Accepted,
			"Tx1*": confirmation.Rejected,
		}))
	}

	// ///////////////////////////////////////////////////////////
	// Accept a Block in epoch 11 -> Epoch 4 becomes committable.
	// ///////////////////////////////////////////////////////////

	{
		epoch11IssuingTime := time.Unix(epoch.GenesisTime+epoch.Duration*10, 0)
		tf.Tangle.CreateBlock("11.A", models.WithStrongParents(tf.Tangle.BlockIDs("1.D")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch11IssuingTime))
		tf.Tangle.CreateBlock("11.B", models.WithStrongParents(tf.Tangle.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch11IssuingTime))
		tf.Tangle.CreateBlock("11.C", models.WithStrongParents(tf.Tangle.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch11IssuingTime))
		tf.Tangle.IssueBlocks("11.A", "11.B", "11.C")
		tf.WaitUntilAllTasksProcessed()

		require.Equal(t, epoch.Index(4), tf.Engine.Storage.Settings.LatestCommitment().Index())
	}

	// ///////////////////////////////////////////////////////////
	// Issue a transaction on epoch 5, spending something created on epoch 1.
	// ///////////////////////////////////////////////////////////

	{
		epocht5IssuingTime := time.Unix(epoch.GenesisTime+epoch.Duration*4, 0)
		tf.Tangle.CreateBlock("5.Z", models.WithStrongParents(tf.Tangle.BlockIDs("1.D")), models.WithPayload(tf.Tangle.CreateTransaction("Tx5", 2, "Tx1.0")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(epocht5IssuingTime))
	}

	// ///////////////////////////////////////////////////////////
	// Accept a Block in epoch 12 -> Epoch 5 becomes committable.
	// ///////////////////////////////////////////////////////////

	{
		epoch12IssuingTime := time.Unix(epoch.GenesisTime+epoch.Duration*11, 0)
		tf.Tangle.CreateBlock("12.A.2", models.WithStrongParents(tf.Tangle.BlockIDs("5.Z")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch12IssuingTime))
		tf.Tangle.CreateBlock("12.B.2", models.WithStrongParents(tf.Tangle.BlockIDs("12.A.2")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch12IssuingTime))
		tf.Tangle.CreateBlock("12.C.2", models.WithStrongParents(tf.Tangle.BlockIDs("12.B.2")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch12IssuingTime))
		tf.Tangle.IssueBlocks("5.Z", "12.A.2", "12.B.2", "12.C.2")
		tf.WaitUntilAllTasksProcessed()

		require.Equal(t, epoch.Index(5), tf.Engine.Storage.Settings.LatestCommitment().Index())
	}

	// ///////////////////////////////////////////////////////////
	// Rollback and Engine to epoch 1, the spent outputs for epoch 2 should be available again.
	// ///////////////////////////////////////////////////////////

	{
		require.NoError(t, tf.Engine.WriteSnapshot(tempDir.Path("snapshot_epoch1.bin"), 1))

		tf2 := NewEngineTestFramework(t)
		require.NoError(t, tf2.Engine.Initialize(tempDir.Path("snapshot_epoch1.bin")))

		require.Equal(t, epoch.Index(1), tf2.Engine.Storage.Settings.LatestCommitment().Index())

		require.True(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx1.0")))
		require.True(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx1.1")))
		require.False(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx5.0")))
		require.False(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx5.1")))

		tf2.WaitUntilAllTasksProcessed()
	}

	// ///////////////////////////////////////////////////////////
	// Stop and start the main engine.
	// ///////////////////////////////////////////////////////////

	{
		tf.WaitUntilAllTasksProcessed()

		expectedBalanceByIDs := tf.Engine.ThroughputQuota.BalanceByIDs()
		expectedTotalBalance := tf.Engine.ThroughputQuota.TotalBalance()

		tf.Engine.Shutdown()
		storageInstance.Shutdown()

		storageInstance := storage.New(storageDir, DatabaseVersion, database.WithDBProvider(database.NewDB))
		t.Cleanup(storageInstance.Shutdown)

		fmt.Println("============================= Start Engine =============================")

		tf2 := NewEngineTestFramework(t, WithStorage(storageInstance), WithTangleOptions(tf.optsTangleOptions...))
		require.NoError(t, tf2.Engine.Initialize(""))

		tf2.AssertEpochState(5)

		require.False(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx1.0")))
		require.True(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx1.1")))
		require.True(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx5.0")))
		require.True(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(tf.Tangle.OutputID("Tx5.1")))

		// ThroughputQuota
		require.Equal(t, expectedBalanceByIDs, tf2.Engine.ThroughputQuota.BalanceByIDs())
		require.Equal(t, expectedTotalBalance, tf2.Engine.ThroughputQuota.TotalBalance())

		tf2.WaitUntilAllTasksProcessed()
	}
}

func TestEngine_ShutdownResume(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*15

	storageDir := t.TempDir()
	storageInstance := storage.New(storageDir, DatabaseVersion)

	tf := NewEngineTestFramework(t, WithStorage(storageInstance), WithTangleOptions(
		tangle.WithBookerOptions(
			booker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
			),
		),
	))
	tempDir := utils.NewDirectory(t.TempDir())

	tf.Engine.NotarizationManager.Events.Error.Attach(event.NewClosure(func(err error) {
		panic(err)
	}))

	identitiesMap := map[string]ed25519.PublicKey{
		"A": identity.GenerateIdentity().PublicKey(),
		"B": identity.GenerateIdentity().PublicKey(),
		"C": identity.GenerateIdentity().PublicKey(),
		"D": identity.GenerateIdentity().PublicKey(),
		"Z": identity.GenerateIdentity().PublicKey(),
	}

	identitiesWeights := map[identity.ID]uint64{
		identity.New(identitiesMap["A"]).ID(): 25,
		identity.New(identitiesMap["B"]).ID(): 25,
		identity.New(identitiesMap["C"]).ID(): 25,
		identity.New(identitiesMap["D"]).ID(): 25,
		identity.New(identitiesMap["Z"]).ID(): 0,
	}

	snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights))

	require.NoError(t, tf.Engine.Initialize(tempDir.Path("genesis_snapshot.bin")))

	require.Equal(t, int64(100), tf.Engine.SybilProtection.Validators().TotalWeight())

	tf.Engine.Shutdown()
	storageInstance.Shutdown()

	storageInstance = storage.New(storageDir, DatabaseVersion)

	tf2 := NewEngineTestFramework(t, WithStorage(storageInstance), WithTangleOptions(tf.optsTangleOptions...))
	require.NoError(t, tf2.Engine.Initialize(""))

	tf2.AssertEpochState(0)
}
