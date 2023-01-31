package protocol

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
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

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.GenerateIdentity().PublicKey(): 100,
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

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.New(identitiesMap["A"]).PublicKey(): 30,
		identity.New(identitiesMap["B"]).PublicKey(): 30,
		identity.New(identitiesMap["C"]).PublicKey(): 30,
		identity.New(identitiesMap["D"]).PublicKey(): 10,
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

	// ...but it gets accepted only when 67% of the active weight is reached.
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

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.New(identitiesMap["A"]).PublicKey(): 25,
		identity.New(identitiesMap["B"]).PublicKey(): 25,
		identity.New(identitiesMap["C"]).PublicKey(): 25,
		identity.New(identitiesMap["D"]).PublicKey(): 25,
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
	require.Eventually(t, func() bool {
		return tf.Engine.Storage.Settings.LatestCommitment().Index() == epoch.Index(4)
	}, time.Second, 100*time.Millisecond)

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

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.New(identitiesMap["A"]).PublicKey(): 25,
		identity.New(identitiesMap["B"]).PublicKey(): 25,
		identity.New(identitiesMap["C"]).PublicKey(): 25,
		identity.New(identitiesMap["D"]).PublicKey(): 25,
		identity.New(identitiesMap["Z"]).PublicKey(): 0,
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

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.New(identitiesMap["A"]).PublicKey(): 25,
		identity.New(identitiesMap["B"]).PublicKey(): 25,
		identity.New(identitiesMap["C"]).PublicKey(): 25,
		identity.New(identitiesMap["D"]).PublicKey(): 25,
		identity.New(identitiesMap["Z"]).PublicKey(): 0,
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

type NodeOnMockedNetwork struct {
	Testing             *testing.T
	KeyPair             ed25519.KeyPair
	Identity            *identity.Identity
	Endpoint            *network.MockedEndpoint
	Protocol            *Protocol
	EngineTestFramework *EngineTestFramework
}

func newNode(t *testing.T, keyPair ed25519.KeyPair, network *network.MockedNetwork, partition string, snapshotPath string, engineOpts ...options.Option[engine.Engine]) *NodeOnMockedNetwork {
	identity := identity.New(keyPair.PublicKey)
	endpoint := network.Join(identity.ID(), partition)
	tempDir := utils.NewDirectory(t.TempDir())

	protocol := New(endpoint,
		WithBaseDirectory(tempDir.Path()),
		WithSnapshotPath(snapshotPath),
		WithEngineOptions(engineOpts...),
	)
	protocol.Run()

	t.Cleanup(func() {
		protocol.Shutdown()
		protocol.CongestionControl.WorkerPool().ShutdownComplete.Wait()
		for _, pool := range protocol.Engine().WorkerPools() {
			pool.ShutdownComplete.Wait()
		}
	})

	tf := NewEngineTestFramework(t, WithEngine(protocol.Engine()), WithTangleOptions(
		tangle.WithBookerOptions(
			booker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
			),
		),
	))

	return &NodeOnMockedNetwork{
		Testing:             t,
		Identity:            identity,
		KeyPair:             keyPair,
		Endpoint:            endpoint,
		Protocol:            protocol,
		EngineTestFramework: tf,
	}
}

func (n *NodeOnMockedNetwork) HookLogging(includeMainEngine bool) {
	events := n.Protocol.Events

	if includeMainEngine {
		n.attachEngineLogs(n.Protocol.mainEngine)
	}

	events.CandidateEngineActivated.Hook(event.NewClosure(func(candidateEngine *enginemanager.EngineInstance) {
		fmt.Printf("%s> CandidateEngineActivated: latest commitment %s %s\n", n.Identity.ID(), candidateEngine.Storage.Settings.LatestCommitment().ID(), candidateEngine.Storage.Settings.LatestCommitment())
		fmt.Printf("==================\nACTIVATE %s\n==================\n", n.Identity.ID())
		n.attachEngineLogs(candidateEngine)
	}))

	events.MainEngineSwitched.Hook(event.NewClosure(func(engine *enginemanager.EngineInstance) {
		fmt.Printf("%s> MainEngineSwitched: latest commitment %s %s\n", n.Identity.ID(), engine.Storage.Settings.LatestCommitment().ID(), engine.Storage.Settings.LatestCommitment())
		fmt.Printf("================\nSWITCH %s\n================\n", n.Identity.ID())
	}))

	events.CongestionControl.Scheduler.BlockScheduled.Hook(event.NewClosure(func(block *scheduler.Block) {
		fmt.Printf("%s> CongestionControl.Scheduler.BlockScheduled: %s\n", n.Identity.ID(), block.ID())
	}))

	events.CongestionControl.Scheduler.BlockDropped.Hook(event.NewClosure(func(block *scheduler.Block) {
		fmt.Printf("%s> CongestionControl.Scheduler.BlockDropped: %s\n", n.Identity.ID(), block.ID())
	}))

	events.CongestionControl.Scheduler.BlockSubmitted.Hook(event.NewClosure(func(block *scheduler.Block) {
		fmt.Printf("%s> CongestionControl.Scheduler.BlockSubmitted: %s\n", n.Identity.ID(), block.ID())
	}))

	events.CongestionControl.Scheduler.BlockSkipped.Hook(event.NewClosure(func(block *scheduler.Block) {
		fmt.Printf("%s> CongestionControl.Scheduler.BlockSkipped: %s\n", n.Identity.ID(), block.ID())
	}))

	events.ChainManager.ForkDetected.Hook(event.NewClosure(func(event *chainmanager.ForkDetectedEvent) {
		fmt.Printf("%s> ChainManager.ForkDetected: %s with forking point %s received from %s\n", n.Identity.ID(), event.Commitment.ID(), event.ForkingPointAgainstMainChain.ID(), event.Source)
		fmt.Printf("----------------------\nForkDetected %s\n----------------------\n", n.Identity.ID())
	}))

	events.Error.Hook(event.NewClosure(func(err error) {
		fmt.Printf("%s> Error: %s\n", n.Identity.ID(), err.Error())
	}))

	events.Network.BlockReceived.Hook(event.NewClosure(func(event *network.BlockReceivedEvent) {
		fmt.Printf("%s> Network.BlockReceived: from %s %s - %d\n", n.Identity.ID(), event.Source, event.Block.ID(), event.Block.ID().Index())
	}))

	events.Network.BlockRequestReceived.Hook(event.NewClosure(func(event *network.BlockRequestReceivedEvent) {
		fmt.Printf("%s> Network.BlockRequestReceived: from %s %s\n", n.Identity.ID(), event.Source, event.BlockID)
	}))

	events.Network.AttestationsReceived.Hook(event.NewClosure(func(event *network.AttestationsReceivedEvent) {
		fmt.Printf("%s> Network.AttestationsReceived: from %s for %s\n", n.Identity.ID(), event.Source, event.Commitment.ID())
	}))

	events.Network.AttestationsRequestReceived.Hook(event.NewClosure(func(event *network.AttestationsRequestReceivedEvent) {
		fmt.Printf("%s> Network.AttestationsRequestReceived: from %s %s -> %d\n", n.Identity.ID(), event.Source, event.Commitment.ID(), event.EndIndex)
	}))

	events.Network.EpochCommitmentReceived.Hook(event.NewClosure(func(event *network.EpochCommitmentReceivedEvent) {
		fmt.Printf("%s> Network.EpochCommitmentReceived: from %s %s\n", n.Identity.ID(), event.Source, event.Commitment.ID())
	}))

	events.Network.EpochCommitmentRequestReceived.Hook(event.NewClosure(func(event *network.EpochCommitmentRequestReceivedEvent) {
		fmt.Printf("%s> Network.EpochCommitmentRequestReceived: from %s %s\n", n.Identity.ID(), event.Source, event.CommitmentID)
	}))

	events.Network.Error.Hook(event.NewClosure(func(event *network.ErrorEvent) {
		fmt.Printf("%s> Network.Error: from %s %s\n", n.Identity.ID(), event.Source, event.Error.Error())
	}))
}

func (n *NodeOnMockedNetwork) attachEngineLogs(instance *enginemanager.EngineInstance) {
	engineName := instance.Name()
	events := instance.Engine.Events

	events.Tangle.BlockDAG.BlockAttached.Hook(event.NewClosure(func(block *blockdag.Block) {
		fmt.Printf("%s> [%s] BlockDAG.BlockAttached: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Tangle.BlockDAG.BlockSolid.Hook(event.NewClosure(func(block *blockdag.Block) {
		fmt.Printf("%s> [%s] BlockDAG.BlockSolid: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Tangle.BlockDAG.BlockInvalid.Hook(event.NewClosure(func(event *blockdag.BlockInvalidEvent) {
		fmt.Printf("%s> [%s] BlockDAG.BlockInvalid: %s - %s\n", n.Identity.ID(), engineName, event.Block.ID(), event.Reason.Error())
	}))

	events.Tangle.BlockDAG.BlockMissing.Hook(event.NewClosure(func(block *blockdag.Block) {
		fmt.Printf("%s> [%s] BlockDAG.BlockMissing: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Tangle.BlockDAG.MissingBlockAttached.Hook(event.NewClosure(func(block *blockdag.Block) {
		fmt.Printf("%s> [%s] BlockDAG.MissingBlockAttached: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Tangle.BlockDAG.BlockOrphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		fmt.Printf("%s> [%s] BlockDAG.BlockOrphaned: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Tangle.BlockDAG.BlockUnorphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		fmt.Printf("%s> [%s] BlockDAG.BlockUnorphaned: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Tangle.Booker.BlockBooked.Hook(event.NewClosure(func(block *booker.Block) {
		fmt.Printf("%s> [%s] Booker.BlockBooked: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Tangle.VirtualVoting.SequenceTracker.VotersUpdated.Hook(event.NewClosure(func(event *sequencetracker.VoterUpdatedEvent) {
		fmt.Printf("%s> [%s] Tangle.VirtualVoting.SequenceTracker.VotersUpdated: %s %s %d -> %d\n", n.Identity.ID(), engineName, event.Voter, event.SequenceID, event.PrevMaxSupportedIndex, event.NewMaxSupportedIndex)
	}))

	events.Clock.AcceptanceTimeUpdated.Hook(event.NewClosure(func(event *clock.TimeUpdateEvent) {
		fmt.Printf("%s> [%s] Clock.AcceptanceTimeUpdated: %s\n", n.Identity.ID(), engineName, event.NewTime)
	}))

	events.Filter.BlockAllowed.Hook(event.NewClosure(func(block *models.Block) {
		fmt.Printf("%s> [%s] Filter.BlockAllowed: %s\n", n.Identity.ID(), engineName, block.ID())
	}))

	events.Filter.BlockFiltered.Hook(event.NewClosure(func(event *filter.BlockFilteredEvent) {
		fmt.Printf("%s> [%s] Filter.BlockFiltered: %s - %s\n", n.Identity.ID(), engineName, event.Block.ID(), event.Reason.Error())
		n.Testing.Fatal("no blocks should be filtered")
	}))

	events.BlockRequester.Tick.Hook(event.NewClosure(func(blockID models.BlockID) {
		fmt.Printf("%s> [%s] BlockRequester.Tick: %s\n", n.Identity.ID(), engineName, blockID)
	}))

	events.BlockProcessed.Hook(event.NewClosure(func(blockID models.BlockID) {
		fmt.Printf("%s> [%s] Engine.BlockProcessed: %s\n", n.Identity.ID(), engineName, blockID)
	}))

	events.Error.Hook(event.NewClosure(func(err error) {
		fmt.Printf("%s> [%s] Engine.Error: %s\n", n.Identity.ID(), engineName, err.Error())
	}))

	events.NotarizationManager.EpochCommitted.Hook(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
		fmt.Printf("%s> [%s] NotarizationManager.EpochCommitted: %s %s\n", n.Identity.ID(), engineName, details.Commitment.ID(), details.Commitment)
	}))

	events.Consensus.BlockGadget.BlockAccepted.Hook(event.NewClosure(func(block *blockgadget.Block) {
		fmt.Printf("%s> [%s] Consensus.BlockGadget.BlockAccepted: %s %s\n", n.Identity.ID(), engineName, block.ID(), block.Commitment().ID())
	}))

	events.Consensus.BlockGadget.BlockConfirmed.Hook(event.NewClosure(func(block *blockgadget.Block) {
		fmt.Printf("%s> [%s] Consensus.BlockGadget.BlockConfirmed: %s %s\n", n.Identity.ID(), engineName, block.ID(), block.Commitment().ID())
	}))

	events.Consensus.EpochGadget.EpochConfirmed.Hook(event.NewClosure(func(epochIndex epoch.Index) {
		fmt.Printf("%s> [%s] Consensus.EpochGadget.EpochConfirmed: %s\n", n.Identity.ID(), engineName, epochIndex)
	}))
}

func (n *NodeOnMockedNetwork) WaitUntilAllTasksProcessed() {
	n.Protocol.WaitWorkerPoolsEmpty()
	n.EngineTestFramework.WaitUntilAllTasksProcessed()
	//TODO: find the race condition in the pools and remove this wait
	time.Sleep(100 * time.Millisecond)
}

func (n *NodeOnMockedNetwork) IssueBlockAtEpoch(alias string, epochIndex epoch.Index, parents ...models.BlockID) *booker.Block {
	issuingTime := time.Unix(epoch.GenesisTime+int64(epochIndex-1)*epoch.Duration, 0)
	require.True(n.Testing, issuingTime.Before(time.Now()), "issued block is in the current or future epoch")
	n.EngineTestFramework.Tangle.CreateAndSignBlock(alias, &n.KeyPair,
		models.WithStrongParents(models.NewBlockIDs(parents...)),
		models.WithIssuingTime(issuingTime),
		models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
	)
	n.EngineTestFramework.Tangle.IssueBlocks(alias)
	n.WaitUntilAllTasksProcessed()
	return n.EngineTestFramework.Tangle.Block(alias)
}

func (n *NodeOnMockedNetwork) IssueBlock(alias string, parents ...models.BlockID) *booker.Block {
	n.EngineTestFramework.Tangle.CreateAndSignBlock(alias, &n.KeyPair,
		models.WithStrongParents(models.NewBlockIDs(parents...)),
		models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
	)
	n.EngineTestFramework.Tangle.IssueBlocks(alias)
	n.WaitUntilAllTasksProcessed()
	return n.EngineTestFramework.Tangle.Block(alias)
}

func (n *NodeOnMockedNetwork) IssueActivity(duration time.Duration) {
	go func() {
		start := time.Now()
		var counter int
		for {
			tips := n.Protocol.TipManager.Tips(1)
			n.issueActivityBlock(fmt.Sprintf("%s.%d", n.Identity.ID(), counter), tips.Slice()...)
			counter++
			time.Sleep(1 * time.Second)
			if duration > 0 && time.Since(start) > duration {
				return
			}
		}
	}()
}

func (n *NodeOnMockedNetwork) issueActivityBlock(alias string, parents ...models.BlockID) {
	n.EngineTestFramework.Tangle.CreateAndSignBlock(alias, &n.KeyPair,
		models.WithStrongParents(models.NewBlockIDs(parents...)),
		models.WithCommitment(n.Protocol.Engine().Storage.Settings.LatestCommitment()),
	)
	n.EngineTestFramework.Tangle.IssueBlocks(alias)
}

func (n *NodeOnMockedNetwork) ValidateAcceptedBlocks(expectedAcceptedBlocks map[models.BlockID]bool) {
	for blockID, blockExpectedAccepted := range expectedAcceptedBlocks {
		actualBlockAccepted := n.Protocol.Engine().Consensus.BlockGadget.IsBlockAccepted(blockID)
		require.Equal(n.Testing, blockExpectedAccepted, actualBlockAccepted, "Block %s should be accepted=%t but is %t", blockID, blockExpectedAccepted, actualBlockAccepted)
	}
}

func (n *NodeOnMockedNetwork) AssertEqualChainsAtLeastAtEpoch(index epoch.Index, other *NodeOnMockedNetwork) {
	lastCommitment := n.Protocol.Engine().Storage.Settings.LatestCommitment()
	otherLastCommitment := other.Protocol.Engine().Storage.Settings.LatestCommitment()

	require.GreaterOrEqual(n.Testing, lastCommitment.Index(), index)
	require.GreaterOrEqual(n.Testing, otherLastCommitment.Index(), index)

	oldestIndex := lo.Min(lastCommitment.Index(), otherLastCommitment.Index())
	require.Equal(n.Testing, lo.PanicOnErr(n.Protocol.Engine().Storage.Commitments.Load(oldestIndex)), lo.PanicOnErr(other.Protocol.Engine().Storage.Commitments.Load(oldestIndex)))
}

func TestProtocol_EngineSwitching(t *testing.T) {
	testNetwork := network.NewMockedNetwork()

	engineOptions := []options.Option[engine.Engine]{
		engine.WithNotarizationManagerOptions(
			notarization.WithMinCommittableEpochAge(10 * time.Second),
		),
		engine.WithLedgerOptions(
			ledger.WithVM(new(devnetvm.VM)),
		),
	}

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*10

	identitiesMap := map[string]ed25519.KeyPair{
		"node1": ed25519.GenerateKeyPair(),
		"node2": ed25519.GenerateKeyPair(),
		"node3": ed25519.GenerateKeyPair(),
		"node4": ed25519.GenerateKeyPair(),
	}

	for alias, keyPair := range identitiesMap {
		identity.RegisterIDAlias(identity.NewID(keyPair.PublicKey), alias)
	}

	partition1Weights := map[ed25519.PublicKey]uint64{
		identitiesMap["node1"].PublicKey: 75,
		identitiesMap["node2"].PublicKey: 75,
	}

	partition2Weights := map[ed25519.PublicKey]uint64{
		identitiesMap["node3"].PublicKey: 25,
		identitiesMap["node4"].PublicKey: 25,
	}

	allWeights := map[ed25519.PublicKey]uint64{}
	lo.MergeMaps(allWeights, partition1Weights)
	lo.MergeMaps(allWeights, partition2Weights)

	snapshotsDir := utils.NewDirectory(t.TempDir())
	snapshot := snapshotsDir.Path("snapshot.bin")
	snapshotcreator.CreateSnapshot(DatabaseVersion, snapshot, 0, make([]byte, 32), allWeights, nil, engineOptions...)

	node1 := newNode(t, identitiesMap["node1"], testNetwork, "P1", snapshot, engineOptions...)
	node2 := newNode(t, identitiesMap["node2"], testNetwork, "P1", snapshot, engineOptions...)
	node3 := newNode(t, identitiesMap["node3"], testNetwork, "P2", snapshot, engineOptions...)
	node4 := newNode(t, identitiesMap["node4"], testNetwork, "P2", snapshot, engineOptions...)

	node1.HookLogging(true)
	node2.HookLogging(true)
	node3.HookLogging(true)
	node4.HookLogging(true)

	// Verify all nodes have the expected state
	{
		// Partition 1
		require.Equal(t, int64(200), node1.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(0), node1.Protocol.Engine().SybilProtection.Validators().TotalWeight())
		require.Equal(t, int64(200), node2.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(0), node2.Protocol.Engine().SybilProtection.Validators().TotalWeight())

		// Partition 2
		require.Equal(t, int64(200), node3.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(0), node3.Protocol.Engine().SybilProtection.Validators().TotalWeight())
		require.Equal(t, int64(200), node4.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(0), node4.Protocol.Engine().SybilProtection.Validators().TotalWeight())

		// Add the validators manually to the active set of each partition
		for key := range partition1Weights {
			node1.Protocol.Engine().SybilProtection.Validators().Add(identity.NewID(key))
			node2.Protocol.Engine().SybilProtection.Validators().Add(identity.NewID(key))
		}

		for key := range partition2Weights {
			node3.Protocol.Engine().SybilProtection.Validators().Add(identity.NewID(key))
			node4.Protocol.Engine().SybilProtection.Validators().Add(identity.NewID(key))
		}

		// Partition 1
		require.Equal(t, int64(200), node1.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(150), node1.Protocol.Engine().SybilProtection.Validators().TotalWeight())
		require.Equal(t, int64(200), node2.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(150), node2.Protocol.Engine().SybilProtection.Validators().TotalWeight())

		// Partition 2
		require.Equal(t, int64(200), node3.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(50), node3.Protocol.Engine().SybilProtection.Validators().TotalWeight())
		require.Equal(t, int64(200), node4.Protocol.Engine().SybilProtection.Weights().TotalWeight())
		require.Equal(t, int64(50), node4.Protocol.Engine().SybilProtection.Validators().TotalWeight())

		// All are at epoch 0 with the same commitment
		require.Equal(t, epoch.Index(0), node1.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
		require.Equal(t, epoch.Index(0), node2.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
		require.Equal(t, epoch.Index(0), node3.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
		require.Equal(t, epoch.Index(0), node4.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
		require.Equal(t, node1.Protocol.Engine().Storage.Settings.LatestCommitment(), node2.Protocol.Engine().Storage.Settings.LatestCommitment())
		require.Equal(t, node1.Protocol.Engine().Storage.Settings.LatestCommitment(), node3.Protocol.Engine().Storage.Settings.LatestCommitment())
		require.Equal(t, node1.Protocol.Engine().Storage.Settings.LatestCommitment(), node4.Protocol.Engine().Storage.Settings.LatestCommitment())
	}

	waitOnAllNodes := func() {
		node1.WaitUntilAllTasksProcessed()
		node2.WaitUntilAllTasksProcessed()
		node3.WaitUntilAllTasksProcessed()
		node4.WaitUntilAllTasksProcessed()
	}

	assertBlockExistsOnNodes := func(id models.BlockID, nodes ...*NodeOnMockedNetwork) {
		for _, node := range nodes {
			require.True(t, lo.Return2(node.EngineTestFramework.Engine.Block(id)))
		}
	}

	assertBlockMissingOnNodes := func(id models.BlockID, nodes ...*NodeOnMockedNetwork) {
		for _, node := range nodes {
			require.False(t, lo.Return2(node.EngineTestFramework.Engine.Block(id)))
		}
	}

	genesisBlockID := node1.EngineTestFramework.Tangle.Block("Genesis")

	// Issue blocks on Partition 1
	{
		blockA := node1.IssueBlockAtEpoch("P1.A", 5, genesisBlockID.ID())
		waitOnAllNodes()

		blockB := node2.IssueBlockAtEpoch("P1.B", 6, blockA.ID())
		waitOnAllNodes()

		blockC := node1.IssueBlockAtEpoch("P1.C", 7, blockB.ID())
		waitOnAllNodes()

		blockD := node2.IssueBlockAtEpoch("P1.D", 8, blockC.ID())
		waitOnAllNodes()

		blockE := node1.IssueBlockAtEpoch("P1.E", 9, blockD.ID())
		waitOnAllNodes()

		blockF := node2.IssueBlockAtEpoch("P1.F", 10, blockE.ID())
		waitOnAllNodes()

		blockG := node1.IssueBlockAtEpoch("P1.G", 11, blockF.ID())
		waitOnAllNodes()

		assertBlockExistsOnNodes(blockA.ID(), node1, node2)
		assertBlockExistsOnNodes(blockB.ID(), node1, node2)
		assertBlockExistsOnNodes(blockC.ID(), node1, node2)
		assertBlockExistsOnNodes(blockD.ID(), node1, node2)
		assertBlockExistsOnNodes(blockE.ID(), node1, node2)
		assertBlockExistsOnNodes(blockF.ID(), node1, node2)
		assertBlockExistsOnNodes(blockG.ID(), node1, node2)

		assertBlockMissingOnNodes(blockA.ID(), node3, node4)
		assertBlockMissingOnNodes(blockB.ID(), node3, node4)
		assertBlockMissingOnNodes(blockC.ID(), node3, node4)
		assertBlockMissingOnNodes(blockD.ID(), node3, node4)
		assertBlockMissingOnNodes(blockE.ID(), node3, node4)
		assertBlockMissingOnNodes(blockF.ID(), node3, node4)
		assertBlockMissingOnNodes(blockG.ID(), node3, node4)

		acceptedBlocks := map[models.BlockID]bool{
			blockA.ID(): true,
			blockB.ID(): true,
			blockC.ID(): true,
			blockD.ID(): true,
			blockE.ID(): true,
			blockF.ID(): true,
			blockG.ID(): false, // block not referenced yet
		}

		node1.ValidateAcceptedBlocks(acceptedBlocks)
		node2.ValidateAcceptedBlocks(acceptedBlocks)
	}

	// Issue blocks on Partition 2
	partition2Tips := models.NewBlockIDs()
	{
		blockA := node3.IssueBlockAtEpoch("P2.A", 5, genesisBlockID.ID())
		waitOnAllNodes()

		blockB := node4.IssueBlockAtEpoch("P2.B", 6, blockA.ID())
		waitOnAllNodes()

		blockC := node3.IssueBlockAtEpoch("P2.C", 7, blockB.ID())
		waitOnAllNodes()

		blockD := node4.IssueBlockAtEpoch("P2.D", 8, blockC.ID())
		waitOnAllNodes()

		blockE := node3.IssueBlockAtEpoch("P2.E", 9, blockD.ID())
		waitOnAllNodes()

		blockF := node4.IssueBlockAtEpoch("P2.E", 10, blockE.ID())
		waitOnAllNodes()

		blockG := node3.IssueBlockAtEpoch("P2.E", 11, blockF.ID())
		waitOnAllNodes()

		assertBlockExistsOnNodes(blockA.ID(), node3, node4)
		assertBlockExistsOnNodes(blockB.ID(), node3, node4)
		assertBlockExistsOnNodes(blockC.ID(), node3, node4)
		assertBlockExistsOnNodes(blockD.ID(), node3, node4)
		assertBlockExistsOnNodes(blockE.ID(), node3, node4)
		assertBlockExistsOnNodes(blockF.ID(), node3, node4)
		assertBlockExistsOnNodes(blockG.ID(), node3, node4)

		assertBlockMissingOnNodes(blockA.ID(), node1, node2)
		assertBlockMissingOnNodes(blockB.ID(), node1, node2)
		assertBlockMissingOnNodes(blockC.ID(), node1, node2)
		assertBlockMissingOnNodes(blockD.ID(), node1, node2)
		assertBlockMissingOnNodes(blockE.ID(), node1, node2)
		assertBlockMissingOnNodes(blockF.ID(), node1, node2)
		assertBlockMissingOnNodes(blockG.ID(), node1, node2)

		acceptedBlocks := map[models.BlockID]bool{
			blockA.ID(): true,
			blockB.ID(): true,
			blockC.ID(): true,
			blockD.ID(): true,
			blockE.ID(): true,
			blockF.ID(): true,
			blockG.ID(): false, // block not referenced yet
		}

		node3.ValidateAcceptedBlocks(acceptedBlocks)
		node4.ValidateAcceptedBlocks(acceptedBlocks)

		partition2Tips.Add(blockD.ID())
	}

	// Both partitions should have committed epoch 8 and have different commitments
	{
		waitOnAllNodes()
		require.Equal(t, epoch.Index(8), node1.Protocol.Engine().Storage.Settings.LatestCommitment().Index())
		require.Equal(t, epoch.Index(8), node2.Protocol.Engine().Storage.Settings.LatestCommitment().Index())
		require.Equal(t, epoch.Index(8), node3.Protocol.Engine().Storage.Settings.LatestCommitment().Index())
		require.Equal(t, epoch.Index(8), node4.Protocol.Engine().Storage.Settings.LatestCommitment().Index())

		require.Equal(t, node1.Protocol.Engine().Storage.Settings.LatestCommitment(), node2.Protocol.Engine().Storage.Settings.LatestCommitment())
		require.Equal(t, node3.Protocol.Engine().Storage.Settings.LatestCommitment(), node4.Protocol.Engine().Storage.Settings.LatestCommitment())
		require.NotEqual(t, node1.Protocol.Engine().Storage.Settings.LatestCommitment(), node3.Protocol.Engine().Storage.Settings.LatestCommitment())
	}

	// Merge the partitions
	{
		testNetwork.MergePartitionsToMain()
		println("\n=========================\nMerged network partitions\n=========================\n")
	}

	// Issue blocks after merging the networks
	{
		node1.IssueActivity(20 * time.Second)
		node2.IssueActivity(20 * time.Second)
		node3.IssueActivity(20 * time.Second)
		node4.IssueActivity(20 * time.Second)
	}

	// Wait for the engine to eventually switch on each node
	{
		nodeCount := atomic.NewInt32(0)
		for _, node := range []*NodeOnMockedNetwork{node3, node4} {
			nodeCount.Add(1)
			node.Protocol.Events.MainEngineSwitched.Attach(event.NewClosure(func(_ *enginemanager.EngineInstance) {
				nodeCount.Add(-1)
			}))
		}
		require.Eventually(t, func() bool {
			return nodeCount.Load() == 0
		}, 15*time.Second, 100*time.Millisecond, "not all nodes switched main engine")
	}

	time.Sleep(6 * time.Second)

	// Compare chains
	{
		waitOnAllNodes()
		// Check that all nodes have at least an epoch committed after we merged them at 8 and that they follow the same commitments
		node1.AssertEqualChainsAtLeastAtEpoch(9, node2)
		node1.AssertEqualChainsAtLeastAtEpoch(9, node3)
		node1.AssertEqualChainsAtLeastAtEpoch(9, node4)
	}
}
