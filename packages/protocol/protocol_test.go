package protocol_test

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
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/enginemanager"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/mockednetwork"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
)

func TestProtocol(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	ledgerVM := new(devnetvm.VM)

	workers := workerpool.NewGroup(t.Name())

	testNetwork := network.NewMockedNetwork()

	endpoint1 := testNetwork.Join(identity.GenerateIdentity().ID())

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.GenerateIdentity().PublicKey(): 100,
	}

	tempDir := utils.NewDirectory(t.TempDir())
	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, tempDir.Path("snapshot.bin"), 100, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

	protocol1 := protocol.New(workers.CreateGroup("Protocol1"), endpoint1, protocol.WithBaseDirectory(tempDir.Path()), protocol.WithSnapshotPath(tempDir.Path("snapshot.bin")), protocol.WithEngineOptions(engine.WithLedgerOptions(ledger.WithVM(ledgerVM))))
	protocol1.Run()
	t.Cleanup(protocol1.Shutdown)

	commitments := make(map[string]*commitment.Commitment)
	commitments["0"] = commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	commitments["1"] = commitment.New(1, commitments["0"].ID(), types.Identifier{1}, 0)
	commitments["2"] = commitment.New(2, commitments["1"].ID(), types.Identifier{2}, 0)
	commitments["3"] = commitment.New(3, commitments["2"].ID(), types.Identifier{3}, 0)

	protocol1.Events.Network.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["1"],
		Source:     identity.ID{},
	})

	protocol1.Events.Network.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["2"],
		Source:     identity.ID{},
	})

	protocol1.Events.Network.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Source:     identity.ID{},
	})

	endpoint2 := testNetwork.Join(identity.GenerateIdentity().ID())

	tempDir2 := utils.NewDirectory(t.TempDir())
	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, tempDir2.Path("snapshot.bin"), 100, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

	protocol2 := protocol.New(workers.CreateGroup("Protocol2"), endpoint2, protocol.WithBaseDirectory(tempDir2.Path()), protocol.WithSnapshotPath(tempDir2.Path("snapshot.bin")), protocol.WithEngineOptions(engine.WithLedgerOptions(ledger.WithVM(ledgerVM))))
	protocol2.Run()
	t.Cleanup(protocol2.Shutdown)

	event.Hook(protocol2.Events.ChainManager.CommitmentMissing, func(id commitment.ID) {
		fmt.Println("MISSING", id)
	})
	event.Hook(protocol2.Events.ChainManager.MissingCommitmentReceived, func(id commitment.ID) {
		fmt.Println("MISSING RECEIVED", id)
	})

	protocol2.Events.Network.EpochCommitmentReceived.Trigger(&network.EpochCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Source:     identity.ID{},
	})

	tf1 := engine.NewTestFramework(t, workers.CreateGroup("EngineTest1"), protocol1.Engine())
	_ = engine.NewTestFramework(t, workers.CreateGroup("EngineTest2"), protocol2.Engine())

	tf1.BlockDAG.CreateBlock("A", models.WithStrongParents(tf1.BlockDAG.BlockIDs("Genesis")))
	tf1.BlockDAG.IssueBlocks("A")

	workers.Wait()
}

func TestEngine_NonEmptyInitialValidators(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	ledgerVM := new(devnetvm.VM)

	epoch.GenesisTime = time.Now().Unix()

	workers := workerpool.NewGroup(t.Name())
	tf := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework"), dpos.NewProvider(), mana1.NewProvider(), engine.WithLedgerOptions(ledger.WithVM(ledgerVM)))

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

	tempDir := utils.NewDirectory(t.TempDir())
	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

	require.NoError(t, tf.Instance.Initialize(tempDir.Path("genesis_snapshot.bin")))

	tf.BlockDAG.CreateBlock("1.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["A"]))
	tf.BlockDAG.IssueBlocks("1.A")

	// If the list of validators would be empty, this block will be accepted right away.
	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"1.A": false,
	})

	tf.BlockDAG.CreateBlock("1.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]))
	tf.BlockDAG.IssueBlocks("1.B")

	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"1.A": false,
		"1.B": false,
	})

	tf.BlockDAG.CreateBlock("1.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]))
	tf.BlockDAG.IssueBlocks("1.C")

	// ...but it gets accepted only when 67% of the active weight is reached.
	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"1.A": true,
		"1.B": false,
		"1.C": false,
	})

	workers.Wait()
}

func TestEngine_BlocksForwardAndRollback(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	ledgerVM := new(devnetvm.VM)

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*10
	fmt.Println("> GenesisTime", time.Unix(epoch.GenesisTime, 0))

	workers := workerpool.NewGroup(t.Name())
	tf := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework"), dpos.NewProvider(), mana1.NewProvider(), engine.WithLedgerOptions(ledger.WithVM(ledgerVM)))

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

	tempDir := utils.NewDirectory(t.TempDir())
	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

	require.NoError(t, tf.Instance.Initialize(tempDir.Path("genesis_snapshot.bin")))

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

	require.Equal(t, epoch.IndexFromTime(tf.Booker.Block("11.A").IssuingTime()), epoch.Index(11))

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

	// Dump snapshot for latest committable epoch 4 and check engine equivalence
	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_epoch4.bin")))

		tf2 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework2"), dpos.NewProvider(), mana1.NewProvider(), engine.WithLedgerOptions(ledger.WithVM(ledgerVM)))

		require.NoError(t, tf2.Instance.Initialize(tempDir.Path("snapshot_epoch4.bin")))

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
	}

	// Dump snapshot for epoch 1 and check attestations equivalence
	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_epoch1.bin"), 1))

		tf3 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework3"), dpos.NewProvider(), mana1.NewProvider(), engine.WithLedgerOptions(ledger.WithVM(ledgerVM)))

		require.NoError(t, tf3.Instance.Initialize(tempDir.Path("snapshot_epoch1.bin")))

		require.Equal(t, epoch.Index(4), tf.Instance.Storage.Settings.LatestCommitment().Index())

		tf3.AssertEpochState(1)

		// Check that we only have attestations for epoch 1.
		require.Equal(t, lo.PanicOnErr(tf.Instance.NotarizationManager.Attestations.Get(1)).Root(), lo.PanicOnErr(tf3.Instance.NotarizationManager.Attestations.Get(1)).Root())
		require.Error(t, lo.Return2(tf3.Instance.NotarizationManager.Attestations.Get(2)))
		require.Error(t, lo.Return2(tf3.Instance.NotarizationManager.Attestations.Get(3)))
		require.Error(t, lo.Return2(tf3.Instance.NotarizationManager.Attestations.Get(4)))
		require.NoError(t, lo.PanicOnErr(tf.Instance.NotarizationManager.Attestations.Get(1)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf3.Instance.NotarizationManager.Attestations.Get(1))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		// RootBlocks
		for epochIndex := epoch.Index(0); epochIndex <= 1; epochIndex++ {
			require.NoError(t, tf.Instance.Storage.RootBlocks.Stream(epochIndex, func(rootBlock models.BlockID) error {
				has, err := tf3.Instance.Storage.RootBlocks.Has(rootBlock)
				require.NoError(t, err)
				require.True(t, has)

				return nil
			}))
		}

		// Block in epoch 2, not accepting anything new.
		tf3.BlockDAG.CreateBlock("2.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch2IssuingTime))
		tf3.BlockDAG.IssueBlocks("2.D")

		// Block in epoch 11
		tf3.BlockDAG.CreateBlock("11.A", models.WithStrongParents(tf3.BlockDAG.BlockIDs("2.D")), models.WithIssuer(identitiesMap["A"]))
		tf3.BlockDAG.IssueBlocks("11.A")

		tf3.BlockDAG.CreateBlock("11.B", models.WithStrongParents(tf3.BlockDAG.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]))
		tf3.BlockDAG.CreateBlock("11.C", models.WithStrongParents(tf3.BlockDAG.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]))
		tf3.BlockDAG.IssueBlocks("11.B", "11.C")

		require.Equal(t, epoch.Index(4), tf3.Instance.Storage.Settings.LatestCommitment().Index())

		// Some blocks got evicted, and we have to restart evaluating with a new map
		acceptedBlocks = make(map[string]bool)
		tf3.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
			"2.D":  true,
			"11.A": true,
			"11.B": false,
			"11.C": false,
		}))
	}

	// Dump snapshot for epoch 2 and check equivalence.
	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_epoch2.bin"), 2))

		tf4 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework4"), dpos.NewProvider(), mana1.NewProvider(), engine.WithLedgerOptions(ledger.WithVM(ledgerVM)))

		require.NoError(t, tf4.Instance.Initialize(tempDir.Path("snapshot_epoch2.bin")))

		require.Equal(t, epoch.Index(2), tf4.Instance.Storage.Settings.LatestCommitment().Index())

		tf4.AssertEpochState(2)

		// Check that we only have attestations for epoch 2.
		require.Nil(t, lo.Return2(tf4.Instance.NotarizationManager.Attestations.Get(1)))
		require.NoError(t, lo.Return2(tf4.Instance.NotarizationManager.Attestations.Get(2)))
		require.Error(t, lo.Return2(tf4.Instance.NotarizationManager.Attestations.Get(3)))
		require.Error(t, lo.Return2(tf4.Instance.NotarizationManager.Attestations.Get(4)))
		require.NoError(t, lo.PanicOnErr(tf.Instance.NotarizationManager.Attestations.Get(2)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf4.Instance.NotarizationManager.Attestations.Get(2))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		// RootBlocks
		for epochIndex := epoch.Index(0); epochIndex <= 2; epochIndex++ {
			require.NoError(t, tf.Instance.Storage.RootBlocks.Stream(epochIndex, func(rootBlock models.BlockID) error {
				has, err := tf4.Instance.Storage.RootBlocks.Has(rootBlock)
				require.NoError(t, err)
				require.True(t, has)

				return nil
			}))
		}
	}
	fmt.Println(workers.Root())
}

func TestEngine_TransactionsForwardAndRollback(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	ledgerVM := new(ledger.MockedVM)

	engineOpts := []options.Option[engine.Engine]{
		engine.WithLedgerOptions(ledger.WithVM(ledgerVM)),
		engine.WithTangleOptions(
			tangle.WithBookerOptions(
				booker.WithMarkerManagerOptions(
					markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
				),
			),
		),
	}

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*15

	workers := workerpool.NewGroup(t.Name())

	testDir := t.TempDir()
	engine1Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
	t.Cleanup(func() {
		workers.Wait()
		engine1Storage.Shutdown()
	})

	engine1 := engine.NewTestEngine(t, workers.CreateGroup("Engine1"), engine1Storage, dpos.NewProvider(), mana1.NewProvider(), engineOpts...)
	tf := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework1"), engine1)

	event.Hook(tf.Instance.NotarizationManager.Events.Error, func(err error) {
		t.Fatal(err.Error())
	})

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

	tempDir := utils.NewDirectory(t.TempDir())
	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

	require.NoError(t, tf.Instance.Initialize(tempDir.Path("genesis_snapshot.bin")))

	require.Equal(t, int64(100), tf.Instance.SybilProtection.Validators().TotalWeight())
	require.Equal(t, int64(101), tf.Instance.ThroughputQuota.TotalBalance())

	acceptedBlocks := make(map[string]bool)
	epoch1IssuingTime := time.Unix(epoch.GenesisTime, 0)

	{
		tf.BlockDAG.CreateBlock("1.Z", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("Tx1", 2, "Genesis")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.BlockDAG.CreateBlock("1.Z*", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("Tx1*", 2, "Genesis")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.BlockDAG.CreateBlock("1.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.Z")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.BlockDAG.CreateBlock("1.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.BlockDAG.CreateBlock("1.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.BlockDAG.CreateBlock("1.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.C")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch1IssuingTime))
		tf.BlockDAG.IssueBlocks("1.Z", "1.Z*", "1.A", "1.B", "1.C", "1.D")

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
		tf.BlockDAG.CreateBlock("11.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch11IssuingTime))
		tf.BlockDAG.CreateBlock("11.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch11IssuingTime))
		tf.BlockDAG.CreateBlock("11.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch11IssuingTime))
		tf.BlockDAG.IssueBlocks("11.A", "11.B", "11.C")

		tf.AssertEpochState(4)
	}

	// ///////////////////////////////////////////////////////////
	// Issue a transaction on epoch 5, spending something created on epoch 1.
	// ///////////////////////////////////////////////////////////

	{
		epocht5IssuingTime := time.Unix(epoch.GenesisTime+epoch.Duration*4, 0)
		tf.BlockDAG.CreateBlock("5.Z", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithPayload(tf.Ledger.CreateTransaction("Tx5", 2, "Tx1.0")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(epocht5IssuingTime))
	}

	// ///////////////////////////////////////////////////////////
	// Accept a Block in epoch 12 -> Epoch 5 becomes committable.
	// ///////////////////////////////////////////////////////////

	{
		epoch12IssuingTime := time.Unix(epoch.GenesisTime+epoch.Duration*11, 0)
		tf.BlockDAG.CreateBlock("12.A.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("5.Z")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch12IssuingTime))
		tf.BlockDAG.CreateBlock("12.B.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("12.A.2")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch12IssuingTime))
		tf.BlockDAG.CreateBlock("12.C.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("12.B.2")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch12IssuingTime))
		tf.BlockDAG.IssueBlocks("5.Z", "12.A.2", "12.B.2", "12.C.2")

		tf.AssertEpochState(5)
	}

	// ///////////////////////////////////////////////////////////
	// Rollback and Engine to epoch 1, the spent outputs for epoch 2 should be available again.
	// ///////////////////////////////////////////////////////////

	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_epoch1.bin"), 1))

		tf2 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework2"), dpos.NewProvider(), mana1.NewProvider(), engineOpts...)
		require.NoError(t, tf2.Instance.Initialize(tempDir.Path("snapshot_epoch1.bin")))

		require.Equal(t, epoch.Index(1), tf2.Instance.Storage.Settings.LatestCommitment().Index())
		tf2.AssertEpochState(1)

		require.True(t, tf2.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx1.0")))
		require.True(t, tf2.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx1.1")))
		require.False(t, tf2.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx5.0")))
		require.False(t, tf2.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx5.1")))

		workers.Wait()
	}

	// ///////////////////////////////////////////////////////////
	// Stop and start the main engine.
	// ///////////////////////////////////////////////////////////

	{
		expectedBalanceByIDs := tf.Instance.ThroughputQuota.BalanceByIDs()
		expectedTotalBalance := tf.Instance.ThroughputQuota.TotalBalance()

		tf.AssertEpochState(5)

		tf.Instance.Shutdown()
		engine1Storage.Shutdown()
		workers.Wait()

		fmt.Println("============================= Start Engine =============================")

		engine3Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
		t.Cleanup(func() {
			workers.Wait()
			engine3Storage.Shutdown()
		})

		engine3 := engine.NewTestEngine(t, workers.CreateGroup("Engine3"), engine3Storage, dpos.NewProvider(), mana1.NewProvider(), engineOpts...)
		tf3 := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework3"), engine3)

		require.NoError(t, tf3.Instance.Initialize(""))

		tf3.AssertEpochState(5)

		require.False(t, tf3.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx1.0")))
		require.True(t, tf3.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx1.1")))
		require.True(t, tf3.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx5.0")))
		require.True(t, tf3.Instance.LedgerState.UnspentOutputs.IDs.Has(tf.Ledger.OutputID("Tx5.1")))

		// ThroughputQuota
		require.Equal(t, expectedBalanceByIDs, tf3.Instance.ThroughputQuota.BalanceByIDs())
		require.Equal(t, expectedTotalBalance, tf3.Instance.ThroughputQuota.TotalBalance())

		workers.Wait()
	}
}

func TestEngine_ShutdownResume(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	ledgerVM := new(devnetvm.VM)

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*15

	workers := workerpool.NewGroup(t.Name())

	testDir := t.TempDir()
	engine1Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
	t.Cleanup(func() {
		workers.Wait()
		engine1Storage.Shutdown()
	})

	engine1 := engine.NewTestEngine(t, workers.CreateGroup("Engine"), engine1Storage,
		dpos.NewProvider(),
		mana1.NewProvider(),
		engine.WithLedgerOptions(ledger.WithVM(ledgerVM)),
		engine.WithTangleOptions(
			tangle.WithBookerOptions(
				booker.WithMarkerManagerOptions(
					markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
				),
			),
		),
	)

	tf := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework1"), engine1)

	event.Hook(tf.Instance.NotarizationManager.Events.Error, func(err error) {
		panic(err)
	})

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

	tempDir := utils.NewDirectory(t.TempDir())
	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, tempDir.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

	require.NoError(t, tf.Instance.Initialize(tempDir.Path("genesis_snapshot.bin")))

	require.Equal(t, int64(100), tf.Instance.SybilProtection.Validators().TotalWeight())

	tf.Instance.Shutdown()
	workers.Wait()
	engine1Storage.Shutdown()

	engine2Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
	t.Cleanup(func() {
		workers.Wait()
		engine2Storage.Shutdown()
	})

	engine2 := engine.NewTestEngine(t, workers.CreateGroup("Engine2"), engine2Storage,
		dpos.NewProvider(),
		mana1.NewProvider(),
		engine.WithLedgerOptions(ledger.WithVM(ledgerVM)),
		engine.WithTangleOptions(
			tangle.WithBookerOptions(
				booker.WithMarkerManagerOptions(
					markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
				),
			),
		),
	)

	tf2 := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework2"), engine2)
	require.NoError(t, tf2.Instance.Initialize(""))
	workers.Wait()
	tf2.AssertEpochState(0)
}

func TestProtocol_EngineSwitching(t *testing.T) {
	testNetwork := network.NewMockedNetwork()

	ledgerVM := new(devnetvm.VM)

	engineOpts := []options.Option[engine.Engine]{
		engine.WithNotarizationManagerOptions(
			notarization.WithMinCommittableEpochAge(10 * time.Second),
		),
		engine.WithLedgerOptions(ledger.WithVM(ledgerVM)),
		engine.WithTangleOptions(
			tangle.WithBookerOptions(
				booker.WithMarkerManagerOptions(
					markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
				),
			),
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

	workers := workerpool.NewGroup(t.Name())

	snapshotsDir := utils.NewDirectory(t.TempDir())
	snapshot := snapshotsDir.Path("snapshot.bin")
	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, snapshot, 0, make([]byte, 32), allWeights, nil, ledgerVM)

	node1 := mockednetwork.NewNode(t, identitiesMap["node1"], testNetwork, "P1", snapshot, engineOpts...)
	node2 := mockednetwork.NewNode(t, identitiesMap["node2"], testNetwork, "P1", snapshot, engineOpts...)
	node3 := mockednetwork.NewNode(t, identitiesMap["node3"], testNetwork, "P2", snapshot, engineOpts...)
	node4 := mockednetwork.NewNode(t, identitiesMap["node4"], testNetwork, "P2", snapshot, engineOpts...)

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

	waitOnAllNodes := func(delay ...time.Duration) {
		if len(delay) > 0 {
			node1.Wait()
			node2.Wait()
			node3.Wait()
			node4.Wait()

			time.Sleep(delay[0])
		}

		node1.Wait()
		node2.Wait()
		node3.Wait()
		node4.Wait()
	}

	assertBlockExistsOnNodes := func(id models.BlockID, nodes ...*mockednetwork.Node) {
		for _, node := range nodes {
			require.True(t, lo.Return2(node.EngineTestFramework.Instance.Block(id)))
		}
	}

	assertBlockMissingOnNodes := func(id models.BlockID, nodes ...*mockednetwork.Node) {
		for _, node := range nodes {
			require.False(t, lo.Return2(node.EngineTestFramework.Instance.Block(id)))
		}
	}

	genesisBlockID := node1.EngineTestFramework.BlockDAG.Block("Genesis")

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

		waitOnAllNodes(1 * time.Second) // Give some time for the blocks to arrive over the network

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

		waitOnAllNodes(1 * time.Second) // Give some time for the blocks to arrive over the network

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
		fmt.Println("\n=========================\nMerged network partitions\n=========================")
	}

	// Issue blocks after merging the networks
	{
		node1.IssueActivity(25 * time.Second)
		node2.IssueActivity(25 * time.Second)
		node3.IssueActivity(25 * time.Second)
		node4.IssueActivity(25 * time.Second)
	}

	// Wait for the engine to eventually switch on each node
	{
		nodeCount := atomic.NewInt32(0)
		wp := workers.CreatePool("Activity", 2)
		for _, node := range []*mockednetwork.Node{node3, node4} {
			nodeCount.Add(1)
			event.AttachWithWorkerPool(node.Protocol.Events.MainEngineSwitched, func(_ *enginemanager.EngineInstance) {
				nodeCount.Add(-1)
			}, wp)
		}
		require.Eventually(t, func() bool {
			return nodeCount.Load() == 0
		}, 30*time.Second, 100*time.Millisecond, "not all nodes switched main engine")
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
