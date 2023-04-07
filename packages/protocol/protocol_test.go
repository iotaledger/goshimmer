package protocol_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock/blocktime"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/tangleconsensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter/blockfilter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/realitiesledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxoledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/mockedvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization/slotnotarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/inmemorytangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/mockednetwork"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func TestProtocol(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())

	testNetwork := network.NewMockedNetwork()

	endpoint1 := testNetwork.Join(identity.GenerateIdentity().ID())

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.GenerateIdentity().PublicKey(): 100,
	}
	tempDir := utils.NewDirectory(t.TempDir())

	ledgerProvider := utxoledger.NewProvider()

	err := snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(tempDir.Path("snapshot.bin")),
		snapshotcreator.WithGenesisTokenAmount(100),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(identitiesWeights),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
	)
	require.NoError(t, err)

	protocol1 := protocol.New(workers.CreateGroup("Protocol1"), endpoint1, protocol.WithBaseDirectory(tempDir.Path()), protocol.WithSnapshotPath(tempDir.Path("snapshot.bin")), protocol.WithLedgerProvider(ledgerProvider))
	protocol1.Run()
	t.Cleanup(protocol1.Shutdown)

	commitments := make(map[string]*commitment.Commitment)
	commitments["0"] = commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	commitments["1"] = commitment.New(1, commitments["0"].ID(), types.Identifier{1}, 0)
	commitments["2"] = commitment.New(2, commitments["1"].ID(), types.Identifier{2}, 0)
	commitments["3"] = commitment.New(3, commitments["2"].ID(), types.Identifier{3}, 0)

	protocol1.Events.Network.SlotCommitmentReceived.Trigger(&network.SlotCommitmentReceivedEvent{
		Commitment: commitments["1"],
		Source:     identity.ID{},
	})

	protocol1.Events.Network.SlotCommitmentReceived.Trigger(&network.SlotCommitmentReceivedEvent{
		Commitment: commitments["2"],
		Source:     identity.ID{},
	})

	protocol1.Events.Network.SlotCommitmentReceived.Trigger(&network.SlotCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Source:     identity.ID{},
	})

	endpoint2 := testNetwork.Join(identity.GenerateIdentity().ID())

	tempDir2 := utils.NewDirectory(t.TempDir())
	err = snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(tempDir2.Path("snapshot.bin")),
		snapshotcreator.WithGenesisTokenAmount(100),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(identitiesWeights),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
	)
	require.NoError(t, err)

	protocol2 := protocol.New(workers.CreateGroup("Protocol2"), endpoint2, protocol.WithBaseDirectory(tempDir2.Path()), protocol.WithSnapshotPath(tempDir2.Path("snapshot.bin")), protocol.WithLedgerProvider(ledgerProvider))
	protocol2.Run()
	t.Cleanup(protocol2.Shutdown)

	protocol2.Events.ChainManager.CommitmentMissing.Hook(func(id commitment.ID) {
		fmt.Println("MISSING", id)
	})
	protocol2.Events.ChainManager.MissingCommitmentReceived.Hook(func(id commitment.ID) {
		fmt.Println("MISSING RECEIVED", id)
	})

	protocol2.Events.Network.SlotCommitmentReceived.Trigger(&network.SlotCommitmentReceivedEvent{
		Commitment: commitments["3"],
		Source:     identity.ID{},
	})

	tf1 := engine.NewTestFramework(t, workers.CreateGroup("EngineTest1"), protocol1.Engine())
	_ = engine.NewTestFramework(t, workers.CreateGroup("EngineTest2"), protocol2.Engine())

	tf1.BlockDAG.CreateBlock("A", models.WithStrongParents(tf1.BlockDAG.BlockIDs("Genesis")))
	tf1.BlockDAG.IssueBlocks("A")

	workers.WaitChildren()
}

func TestEngine_NonEmptyInitialValidators(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

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

	ledgerProvider := utxoledger.NewProvider()

	tempDir := utils.NewDirectory(t.TempDir())
	err := snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(tempDir.Path("genesis_snapshot.bin")),
		snapshotcreator.WithGenesisTokenAmount(1),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(identitiesWeights),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
		snapshotcreator.WithAttestAll(true),
	)
	require.NoError(t, err)

	workers := workerpool.NewGroup(t.Name())
	tf := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework"),
		blocktime.NewProvider(),
		ledgerProvider,
		blockfilter.NewProvider(),
		dpos.NewProvider(),
		mana1.NewProvider(),
		slotnotarization.NewProvider(),
		inmemorytangle.NewProvider(),
		tangleconsensus.NewProvider(),
	)
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

	workers.WaitChildren()
}

func TestEngine_BlocksForwardAndRollback(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

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

	slotDuration := int64(10)

	ledgerProvider := utxoledger.NewProvider()

	err := snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(tempDir.Path("genesis_snapshot.bin")),
		snapshotcreator.WithGenesisTokenAmount(1),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(identitiesWeights),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
		snapshotcreator.WithGenesisUnixTime(time.Now().Unix()-slotDuration*10),
		snapshotcreator.WithSlotDuration(slotDuration),
		snapshotcreator.WithAttestAll(true),
	)
	require.NoError(t, err)

	workers := workerpool.NewGroup(t.Name())
	tf := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework"),
		blocktime.NewProvider(),
		ledgerProvider,
		blockfilter.NewProvider(),
		dpos.NewProvider(),
		mana1.NewProvider(),
		slotnotarization.NewProvider(),
		inmemorytangle.NewProvider(),
		tangleconsensus.NewProvider(),
	)
	require.NoError(t, tf.Instance.Initialize(tempDir.Path("genesis_snapshot.bin")))

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

	require.Equal(t, tf.SlotTimeProvider().IndexFromTime(tf.Booker.Block("11.A").IssuingTime()), slot.Index(11))

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

	// Dump snapshot for latest committable slot 4 and check engine equivalence
	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_slot4.bin")))

		tf2 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework2"),
			blocktime.NewProvider(),
			ledgerProvider,
			blockfilter.NewProvider(),
			dpos.NewProvider(),
			mana1.NewProvider(),
			slotnotarization.NewProvider(),
			inmemorytangle.NewProvider(),
			tangleconsensus.NewProvider(),
		)

		require.NoError(t, tf2.Instance.Initialize(tempDir.Path("snapshot_slot4.bin")))

		// Settings
		// The ChainID of the new engine should correspond to genesis.
		require.Equal(t, lo.PanicOnErr(tf.Instance.Storage.Commitments.Load(0)).ID(), tf2.Instance.Storage.Settings.ChainID())
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
			rootBlocks := tf.BlockDAG.Blocks("1.D", "2.D")
			var earliestCommitment commitment.ID
			for _, rootBlock := range rootBlocks {
				if earliestCommitment.Index() == 0 || rootBlock.Commitment().Index() < earliestCommitment.Index() {
					earliestCommitment = rootBlock.Commitment().ID()
				}
			}

			tf2.AssertRootBlocks(rootBlocks)
			require.Equal(t, earliestCommitment, tf2.Instance.EvictionState.EarliestRootCommitmentID())
		}

		// UTXOLedger
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
	}

	// Dump snapshot for slot 1 and check attestations equivalence
	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_slot1.bin"), 1))

		tf3 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework3"),
			blocktime.NewProvider(),
			ledgerProvider,
			blockfilter.NewProvider(),
			dpos.NewProvider(),
			mana1.NewProvider(),
			slotnotarization.NewProvider(),
			inmemorytangle.NewProvider(),
			tangleconsensus.NewProvider(),
		)

		require.NoError(t, tf3.Instance.Initialize(tempDir.Path("snapshot_slot1.bin")))

		require.Equal(t, slot.Index(4), tf.Instance.Storage.Settings.LatestCommitment().Index())

		tf3.AssertSlotState(1)

		// Check that we only have attestations for slot 1.
		require.Equal(t, lo.PanicOnErr(tf.Instance.Notarization.Attestations().Get(1)).Root(), lo.PanicOnErr(tf3.Instance.Notarization.Attestations().Get(1)).Root())
		require.Error(t, lo.Return2(tf3.Instance.Notarization.Attestations().Get(2)))
		require.Error(t, lo.Return2(tf3.Instance.Notarization.Attestations().Get(3)))
		require.Error(t, lo.Return2(tf3.Instance.Notarization.Attestations().Get(4)))
		require.NoError(t, lo.PanicOnErr(tf.Instance.Notarization.Attestations().Get(1)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf3.Instance.Notarization.Attestations().Get(1))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		// RootBlocks
		rootBlocks := tf.BlockDAG.Blocks("1.D")
		var earliestCommitment commitment.ID
		for _, rootBlock := range rootBlocks {
			if earliestCommitment.Index() == 0 || rootBlock.Commitment().Index() < earliestCommitment.Index() {
				earliestCommitment = rootBlock.Commitment().ID()
			}
		}

		tf3.AssertRootBlocks(rootBlocks)
		require.Equal(t, earliestCommitment, tf3.Instance.EvictionState.EarliestRootCommitmentID())

		// Block in slot 2, not accepting anything new.
		tf3.BlockDAG.CreateBlock("2.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(slot2IssuingTime))
		tf3.BlockDAG.IssueBlocks("2.D")

		// Block in slot 11
		tf3.BlockDAG.CreateBlock("11.A", models.WithStrongParents(tf3.BlockDAG.BlockIDs("2.D")), models.WithIssuer(identitiesMap["A"]))
		tf3.BlockDAG.IssueBlocks("11.A")

		tf3.BlockDAG.CreateBlock("11.B", models.WithStrongParents(tf3.BlockDAG.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]))
		tf3.BlockDAG.CreateBlock("11.C", models.WithStrongParents(tf3.BlockDAG.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]))
		tf3.BlockDAG.IssueBlocks("11.B", "11.C")

		require.Equal(t, slot.Index(4), tf3.Instance.Storage.Settings.LatestCommitment().Index())

		// Some blocks got evicted, and we have to restart evaluating with a new map
		acceptedBlocks = make(map[string]bool)
		tf3.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
			"2.D":  true,
			"11.A": true,
			"11.B": false,
			"11.C": false,
		}))
	}

	// Dump snapshot for slot 2 and check equivalence.
	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_slot2.bin"), 2))

		tf4 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework4"),
			blocktime.NewProvider(),
			ledgerProvider,
			blockfilter.NewProvider(),
			dpos.NewProvider(),
			mana1.NewProvider(),
			slotnotarization.NewProvider(),
			inmemorytangle.NewProvider(),
			tangleconsensus.NewProvider(),
		)

		require.NoError(t, tf4.Instance.Initialize(tempDir.Path("snapshot_slot2.bin")))

		require.Equal(t, slot.Index(2), tf4.Instance.Storage.Settings.LatestCommitment().Index())

		tf4.AssertSlotState(2)

		// Check that we only have attestations for slot 2.
		require.Nil(t, lo.Return2(tf4.Instance.Notarization.Attestations().Get(1)))
		require.NoError(t, lo.Return2(tf4.Instance.Notarization.Attestations().Get(2)))
		require.Error(t, lo.Return2(tf4.Instance.Notarization.Attestations().Get(3)))
		require.Error(t, lo.Return2(tf4.Instance.Notarization.Attestations().Get(4)))
		require.NoError(t, lo.PanicOnErr(tf.Instance.Notarization.Attestations().Get(2)).Stream(func(key identity.ID, engine1Attestation *notarization.Attestation) bool {
			engine2Attestations := lo.PanicOnErr(tf4.Instance.Notarization.Attestations().Get(2))
			engine2Attestation, exists := engine2Attestations.Get(key)
			require.True(t, exists)
			require.Equal(t, engine1Attestation, engine2Attestation)

			return true
		}))

		// RootBlocks
		for slotIndex := slot.Index(0); slotIndex <= 2; slotIndex++ {
			require.NoError(t, tf.Instance.Storage.RootBlocks.Stream(slotIndex, func(rootBlock models.BlockID, _ commitment.ID) error {
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
	slotDuration := int64(10)

	ledgerProvider := utxoledger.NewProvider(
		utxoledger.WithMemPoolProvider(
			realitiesledger.NewProvider(
				realitiesledger.WithVM(new(mockedvm.MockedVM))),
		),
	)

	err := snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(tempDir.Path("genesis_snapshot.bin")),
		snapshotcreator.WithGenesisTokenAmount(1),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(identitiesWeights),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
		snapshotcreator.WithAttestAll(true),
		snapshotcreator.WithGenesisUnixTime(time.Now().Unix()-slotDuration*15),
		snapshotcreator.WithSlotDuration(slotDuration),
	)
	require.NoError(t, err)

	workers := workerpool.NewGroup(t.Name())

	testDir := t.TempDir()
	engine1Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
	t.Cleanup(func() {
		workers.WaitChildren()
		engine1Storage.Shutdown()
	})

	engine1 := engine.NewTestEngine(t, workers.CreateGroup("Engine1"), engine1Storage,
		blocktime.NewProvider(),
		ledgerProvider,
		blockfilter.NewProvider(),
		dpos.NewProvider(),
		mana1.NewProvider(),
		slotnotarization.NewProvider(),
		inmemorytangle.NewProvider(
			inmemorytangle.WithBookerProvider(
				markerbooker.NewProvider(
					markerbooker.WithMarkerManagerOptions(
						markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
					),
				),
			),
		),
		tangleconsensus.NewProvider(),
	)
	tf := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework1"), engine1)
	require.NoError(t, tf.Instance.Initialize(tempDir.Path("genesis_snapshot.bin")))

	tf.Instance.Events.Notarization.Error.Hook(func(err error) {
		t.Fatal(err.Error())
	})

	require.Equal(t, int64(100), tf.Instance.SybilProtection.Validators().TotalWeight())
	require.Equal(t, int64(101), tf.Instance.ThroughputQuota.TotalBalance())

	acceptedBlocks := make(map[string]bool)
	slot1IssuingTime := tf.SlotTimeProvider().StartTime(1)

	{
		tf.BlockDAG.CreateBlock("1.Z", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.MemPool.CreateTransaction("Tx1", 2, "Genesis")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(slot1IssuingTime))
		tf.BlockDAG.CreateBlock("1.Z*", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.MemPool.CreateTransaction("Tx1*", 2, "Genesis")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(slot1IssuingTime))
		tf.BlockDAG.CreateBlock("1.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.Z")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot1IssuingTime))
		tf.BlockDAG.CreateBlock("1.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot1IssuingTime))
		tf.BlockDAG.CreateBlock("1.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(slot1IssuingTime))
		tf.BlockDAG.CreateBlock("1.D", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.C")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(slot1IssuingTime))
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
	// Accept a Block in slot 11 -> slot 4 becomes committable.
	// ///////////////////////////////////////////////////////////

	{
		slot11IssuingTime := tf.SlotTimeProvider().StartTime(11)
		tf.BlockDAG.CreateBlock("11.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot11IssuingTime))
		tf.BlockDAG.CreateBlock("11.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot11IssuingTime))
		tf.BlockDAG.CreateBlock("11.C", models.WithStrongParents(tf.BlockDAG.BlockIDs("11.B")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(slot11IssuingTime))
		tf.BlockDAG.IssueBlocks("11.A", "11.B", "11.C")

		tf.AssertSlotState(4)
	}

	// ///////////////////////////////////////////////////////////
	// Issue a transaction on slot 5, spending something created on slot 1.
	// ///////////////////////////////////////////////////////////

	{
		slot5IssuingTime := tf.SlotTimeProvider().StartTime(5)
		tf.BlockDAG.CreateBlock("5.Z", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.D")), models.WithPayload(tf.MemPool.CreateTransaction("Tx5", 2, "Tx1.0")), models.WithIssuer(identitiesMap["Z"]), models.WithIssuingTime(slot5IssuingTime))
	}

	// ///////////////////////////////////////////////////////////
	// Accept a Block in slot 12 -> Slot 5 becomes committable.
	// ///////////////////////////////////////////////////////////

	{
		slot12IssuingTime := tf.SlotTimeProvider().StartTime(12)
		tf.BlockDAG.CreateBlock("12.A.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("5.Z")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot12IssuingTime))
		tf.BlockDAG.CreateBlock("12.B.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("12.A.2")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot12IssuingTime))
		tf.BlockDAG.CreateBlock("12.C.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("12.B.2")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(slot12IssuingTime))
		tf.BlockDAG.IssueBlocks("5.Z", "12.A.2", "12.B.2", "12.C.2")

		tf.AssertSlotState(5)
	}

	// ///////////////////////////////////////////////////////////
	// Rollback and Engine to slot 1, the spent outputs for slot 2 should be available again.
	// ///////////////////////////////////////////////////////////

	{
		require.NoError(t, tf.Instance.WriteSnapshot(tempDir.Path("snapshot_slot1.bin"), 1))

		tf2 := engine.NewDefaultTestFramework(t, workers.CreateGroup("EngineTestFramework2"),
			blocktime.NewProvider(),
			ledgerProvider,
			blockfilter.NewProvider(),
			dpos.NewProvider(),
			mana1.NewProvider(),
			slotnotarization.NewProvider(),
			inmemorytangle.NewProvider(
				inmemorytangle.WithBookerProvider(
					markerbooker.NewProvider(
						markerbooker.WithMarkerManagerOptions(
							markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
						),
					),
				),
			),
			tangleconsensus.NewProvider(),
		)
		require.NoError(t, tf2.Instance.Initialize(tempDir.Path("snapshot_slot1.bin")))

		require.Equal(t, slot.Index(1), tf2.Instance.Storage.Settings.LatestCommitment().Index())
		tf2.AssertSlotState(1)

		require.True(t, tf2.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx1.0")))
		require.True(t, tf2.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx1.1")))
		require.False(t, tf2.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx5.0")))
		require.False(t, tf2.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx5.1")))

		workers.WaitChildren()
	}

	// ///////////////////////////////////////////////////////////
	// Stop and start the main engine.
	// ///////////////////////////////////////////////////////////

	{
		expectedBalanceByIDs := tf.Instance.ThroughputQuota.BalanceByIDs()
		expectedTotalBalance := tf.Instance.ThroughputQuota.TotalBalance()

		tf.AssertSlotState(5)

		tf.Instance.Shutdown()
		engine1Storage.Shutdown()
		workers.WaitChildren()

		fmt.Println("============================= Start Engine =============================")

		engine3Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
		t.Cleanup(func() {
			workers.WaitChildren()
			engine3Storage.Shutdown()
		})

		engine3 := engine.NewTestEngine(t, workers.CreateGroup("Engine3"), engine3Storage,
			blocktime.NewProvider(),
			ledgerProvider,
			blockfilter.NewProvider(),
			dpos.NewProvider(),
			mana1.NewProvider(),
			slotnotarization.NewProvider(),
			inmemorytangle.NewProvider(
				inmemorytangle.WithBookerProvider(
					markerbooker.NewProvider(
						markerbooker.WithMarkerManagerOptions(
							markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
						),
					),
				),
			),
			tangleconsensus.NewProvider(),
		)
		tf3 := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework3"), engine3)

		require.NoError(t, tf3.Instance.Initialize())

		tf3.AssertSlotState(5)

		require.False(t, tf3.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx1.0")))
		require.True(t, tf3.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx1.1")))
		require.True(t, tf3.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx5.0")))
		require.True(t, tf3.Instance.Ledger.UnspentOutputs().IDs().Has(tf.MemPool.OutputID("Tx5.1")))

		// ThroughputQuota
		require.Equal(t, expectedBalanceByIDs, tf3.Instance.ThroughputQuota.BalanceByIDs())
		require.Equal(t, expectedTotalBalance, tf3.Instance.ThroughputQuota.TotalBalance())

		workers.WaitChildren()
	}
}

func TestEngine_ShutdownResume(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

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
	slotDuration := int64(10)

	ledgerProvider := utxoledger.NewProvider()

	err := snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(tempDir.Path("genesis_snapshot.bin")),
		snapshotcreator.WithGenesisTokenAmount(1),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(identitiesWeights),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
		snapshotcreator.WithAttestAll(true),
		snapshotcreator.WithGenesisUnixTime(time.Now().Unix()-slotDuration*15),
		snapshotcreator.WithSlotDuration(slotDuration),
	)
	require.NoError(t, err)

	workers := workerpool.NewGroup(t.Name())

	testDir := t.TempDir()
	engine1Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
	t.Cleanup(func() {
		workers.WaitChildren()
		engine1Storage.Shutdown()
	})

	engine1 := engine.NewTestEngine(t, workers.CreateGroup("Engine"), engine1Storage,
		blocktime.NewProvider(),
		ledgerProvider,
		blockfilter.NewProvider(),
		dpos.NewProvider(),
		mana1.NewProvider(),
		slotnotarization.NewProvider(),
		inmemorytangle.NewProvider(
			inmemorytangle.WithBookerProvider(
				markerbooker.NewProvider(
					markerbooker.WithMarkerManagerOptions(
						markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
					),
				),
			),
		),
		tangleconsensus.NewProvider(),
	)

	tf := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework1"), engine1)
	require.NoError(t, tf.Instance.Initialize(tempDir.Path("genesis_snapshot.bin")))

	tf.Instance.Events.Notarization.Error.Hook(func(err error) {
		panic(err)
	})

	require.Equal(t, int64(100), tf.Instance.SybilProtection.Validators().TotalWeight())

	tf.Instance.Shutdown()
	workers.WaitChildren()
	engine1Storage.Shutdown()

	engine2Storage := storage.New(testDir, protocol.DatabaseVersion, database.WithDBProvider(database.NewDB))
	t.Cleanup(func() {
		workers.WaitChildren()
		engine2Storage.Shutdown()
	})

	engine2 := engine.NewTestEngine(t, workers.CreateGroup("Engine2"), engine2Storage,
		blocktime.NewProvider(),
		ledgerProvider,
		blockfilter.NewProvider(),
		dpos.NewProvider(),
		mana1.NewProvider(),
		slotnotarization.NewProvider(),
		inmemorytangle.NewProvider(
			inmemorytangle.WithBookerProvider(
				markerbooker.NewProvider(
					markerbooker.WithMarkerManagerOptions(
						markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(1)),
					),
				),
			),
		),
		tangleconsensus.NewProvider(),
	)

	tf2 := engine.NewTestFramework(t, workers.CreateGroup("EngineTestFramework2"), engine2)
	require.NoError(t, tf2.Instance.Initialize())
	workers.WaitChildren()
	tf2.AssertSlotState(0)

	// TODO: extend this test to actually check if we have rootblocks, attestations, weights, etc.
	// pretty much everything that we import from a snapshot we should populate from disk

	// this fails
	// require.Equal(t, int64(100), tf2.Instance.SybilProtection.Validators().TotalWeight())
}

func TestProtocol_EngineSwitching(t *testing.T) {
	testNetwork := network.NewMockedNetwork()

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
	slotDuration := int64(10)

	ledgerProvider := utxoledger.NewProvider()

	err := snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(snapshotsDir.Path("snapshot.bin")),
		snapshotcreator.WithGenesisTokenAmount(0),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(allWeights),
		snapshotcreator.WithLedgerProvider(ledgerProvider),
		snapshotcreator.WithGenesisUnixTime(time.Now().Unix()-slotDuration*10),
		snapshotcreator.WithSlotDuration(slotDuration),
	)
	require.NoError(t, err)

	node1 := mockednetwork.NewNode(t, identitiesMap["node1"], testNetwork, "P1", snapshot, ledgerProvider)
	node2 := mockednetwork.NewNode(t, identitiesMap["node2"], testNetwork, "P1", snapshot, ledgerProvider)
	node3 := mockednetwork.NewNode(t, identitiesMap["node3"], testNetwork, "P2", snapshot, ledgerProvider)
	node4 := mockednetwork.NewNode(t, identitiesMap["node4"], testNetwork, "P2", snapshot, ledgerProvider)

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

		// All are at slot 0 with the same commitment
		require.Equal(t, slot.Index(0), node1.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
		require.Equal(t, slot.Index(0), node2.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
		require.Equal(t, slot.Index(0), node3.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
		require.Equal(t, slot.Index(0), node4.Protocol.Engine().Storage.Settings.LatestCommitment().ID().Index())
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
			require.True(t, lo.Return2(node.EngineTestFramework().Instance.Block(id)))
		}
	}

	assertBlockMissingOnNodes := func(id models.BlockID, nodes ...*mockednetwork.Node) {
		for _, node := range nodes {
			require.False(t, lo.Return2(node.EngineTestFramework().Instance.Block(id)))
		}
	}

	genesisBlockID := node1.EngineTestFramework().BlockDAG.Block("Genesis")

	// Issue blocks on Partition 1
	{
		blockA := node1.IssueBlockAtSlot("P1.A", 5, genesisBlockID.ID())
		waitOnAllNodes()
		blockB := node2.IssueBlockAtSlot("P1.B", 6, blockA.ID())
		waitOnAllNodes()
		blockC := node1.IssueBlockAtSlot("P1.C", 7, blockB.ID())
		waitOnAllNodes()
		blockD := node2.IssueBlockAtSlot("P1.D", 8, blockC.ID())
		waitOnAllNodes()
		blockE := node1.IssueBlockAtSlot("P1.E", 9, blockD.ID())
		waitOnAllNodes()
		blockF := node2.IssueBlockAtSlot("P1.F", 10, blockE.ID())
		waitOnAllNodes()
		blockG := node1.IssueBlockAtSlot("P1.G", 11, blockF.ID())

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
		blockA := node3.IssueBlockAtSlot("P2.A", 5, genesisBlockID.ID())
		waitOnAllNodes()
		blockB := node4.IssueBlockAtSlot("P2.B", 6, blockA.ID())
		waitOnAllNodes()
		blockC := node3.IssueBlockAtSlot("P2.C", 7, blockB.ID())
		waitOnAllNodes()
		blockD := node4.IssueBlockAtSlot("P2.D", 8, blockC.ID())
		waitOnAllNodes()
		blockE := node3.IssueBlockAtSlot("P2.E", 9, blockD.ID())
		waitOnAllNodes()
		blockF := node4.IssueBlockAtSlot("P2.E", 10, blockE.ID())
		waitOnAllNodes()
		blockG := node3.IssueBlockAtSlot("P2.E", 11, blockF.ID())

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

	// Both partitions should have committed slot 8 and have different commitments
	{
		waitOnAllNodes()
		require.Equal(t, slot.Index(8), node1.Protocol.Engine().Storage.Settings.LatestCommitment().Index())
		require.Equal(t, slot.Index(8), node2.Protocol.Engine().Storage.Settings.LatestCommitment().Index())
		require.Equal(t, slot.Index(8), node3.Protocol.Engine().Storage.Settings.LatestCommitment().Index())
		require.Equal(t, slot.Index(8), node4.Protocol.Engine().Storage.Settings.LatestCommitment().Index())

		require.Equal(t, node1.Protocol.Engine().Storage.Settings.LatestCommitment(), node2.Protocol.Engine().Storage.Settings.LatestCommitment())
		require.Equal(t, node3.Protocol.Engine().Storage.Settings.LatestCommitment(), node4.Protocol.Engine().Storage.Settings.LatestCommitment())
		require.NotEqual(t, node1.Protocol.Engine().Storage.Settings.LatestCommitment(), node3.Protocol.Engine().Storage.Settings.LatestCommitment())
	}

	// Merge the partitions
	{
		testNetwork.MergePartitionsToMain()
		fmt.Println("\n=========================\nMerged network partitions\n=========================")
	}

	wg := &sync.WaitGroup{}

	// Issue blocks after merging the networks
	{
		wg.Add(4)

		node1.IssueActivity(25*time.Second, wg)
		node2.IssueActivity(25*time.Second, wg)
		node3.IssueActivity(25*time.Second, wg)
		node4.IssueActivity(25*time.Second, wg)
	}

	// Wait for the engine to eventually switch on each node
	{
		nodeCount := atomic.NewInt32(0)
		wp := workers.CreatePool("Activity", 2)
		for _, node := range []*mockednetwork.Node{node3, node4} {
			nodeCount.Add(1)
			node.Protocol.Events.MainEngineSwitched.Hook(func(_ *engine.Engine) {
				nodeCount.Add(-1)
			}, event.WithWorkerPool(wp))
		}
		require.Eventually(t, func() bool {
			return nodeCount.Load() == 0
		}, 30*time.Second, 100*time.Millisecond, "not all nodes switched main engine")
	}

	wg.Wait()

	// Compare chains
	{
		waitOnAllNodes()
		// Check that all nodes have at least a slot committed after we merged them at 8 and that they follow the same commitments
		node1.AssertEqualChainsAtLeastAtSlot(9, node2)
		node1.AssertEqualChainsAtLeastAtSlot(9, node3)
		node1.AssertEqualChainsAtLeastAtSlot(9, node4)
	}
}

func TestProtocol_EngineFromSnapshotAndDisk(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	testNetwork := network.NewMockedNetwork()

	identitiesMap := map[string]ed25519.PublicKey{
		"A": identity.GenerateIdentity().PublicKey(),
		"B": identity.GenerateIdentity().PublicKey(),
	}

	identitiesWeights := map[ed25519.PublicKey]uint64{
		identity.New(identitiesMap["A"]).PublicKey(): 50,
		identity.New(identitiesMap["B"]).PublicKey(): 50,
	}

	tempDir := utils.NewDirectory(t.TempDir())
	slotDuration := int64(10)

	snapshot := tempDir.Path("genesis_snapshot.bin")
	err := snapshotcreator.CreateSnapshot(
		snapshotcreator.WithDatabaseVersion(protocol.DatabaseVersion),
		snapshotcreator.WithFilePath(snapshot),
		snapshotcreator.WithGenesisTokenAmount(1),
		snapshotcreator.WithGenesisSeed(make([]byte, 32)),
		snapshotcreator.WithPledgeIDs(identitiesWeights),
		snapshotcreator.WithLedgerProvider(utxoledger.NewProvider()),
		snapshotcreator.WithAttestAll(true),
		snapshotcreator.WithGenesisUnixTime(time.Now().Unix()-slotDuration*15),
		snapshotcreator.WithSlotDuration(slotDuration),
	)
	require.NoError(t, err)

	node1 := mockednetwork.NewNode(t, ed25519.GenerateKeyPair(), testNetwork, "P1", snapshot, utxoledger.NewProvider())

	node1.Protocol.Events.Engine.Notarization.Error.Hook(func(err error) {
		panic(err)
	})

	require.Equal(t, int64(100), node1.Protocol.Engine().SybilProtection.Validators().TotalWeight())

	genesisCommitment := commitment.NewEmptyCommitment()

	require.Equal(t, genesisCommitment.ID(), node1.Protocol.Engine().Storage.Settings.LatestCommitment().ID())
	require.Equal(t, genesisCommitment.ID(), node1.Protocol.Engine().Storage.Settings.ChainID())
	require.Equal(t, genesisCommitment.ID(), node1.Protocol.ChainManager().RootCommitment().ID())

	tf := node1.EngineTestFramework()

	slot1IssuingTime := tf.SlotTimeProvider().StartTime(1)
	slot2IssuingTime := tf.SlotTimeProvider().StartTime(2)

	// Slot 1
	tf.BlockDAG.CreateBlock("1.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot1IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	tf.BlockDAG.CreateBlock("1.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot1IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	tf.BlockDAG.CreateBlock("1.A*", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.B")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot1IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 2
	tf.BlockDAG.CreateBlock("2.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot2IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	tf.BlockDAG.CreateBlock("2.B*", models.WithStrongParents(tf.BlockDAG.BlockIDs("1.A*")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot2IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))

	tf.BlockDAG.IssueBlocks("1.A", "1.B", "1.A*", "2.B", "2.B*")

	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"1.A":  true,
		"1.B":  true,
		"1.A*": true,
		"2.B":  false,
		"2.B*": false,
	})

	slot3IssuingTime := tf.SlotTimeProvider().StartTime(3)
	slot4IssuingTime := tf.SlotTimeProvider().StartTime(4)

	// Slot 3
	tf.BlockDAG.CreateBlock("3.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("2.B", "2.B*")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot3IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 4
	tf.BlockDAG.CreateBlock("4.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("3.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot4IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))

	tf.BlockDAG.IssueBlocks("3.A", "4.B")

	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"2.B":  true,
		"2.B*": true,
		"3.A":  true,
		"4.B":  false,
	})

	rootBlocks := tf.BlockDAG.Blocks("1.A", "1.A*")
	tf.AssertRootBlocks(rootBlocks)

	slot5IssuingTime := tf.SlotTimeProvider().StartTime(5)
	slot6IssuingTime := tf.SlotTimeProvider().StartTime(6)
	slot7IssuingTime := tf.SlotTimeProvider().StartTime(7)
	slot8IssuingTime := tf.SlotTimeProvider().StartTime(8)

	// Slot 5
	tf.BlockDAG.CreateBlock("5.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("4.B")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot5IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 6
	tf.BlockDAG.CreateBlock("6.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("5.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot6IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 7
	tf.BlockDAG.CreateBlock("7.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("6.B")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot7IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 8
	tf.BlockDAG.CreateBlock("8.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("7.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot8IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))

	tf.BlockDAG.IssueBlocks("5.A", "6.B", "7.A", "8.B")

	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"4.B": true,
		"5.A": true,
		"6.B": true,
		"7.A": true,
		"8.B": false,
	})

	// We evicted rootblocks of epoch 1, as the rootBlock delay is 4, and we are committing 5.
	rootBlocks = tf.BlockDAG.Blocks("3.A", "2.B*", "2.B")
	tf.AssertRootBlocks(rootBlocks)

	// This node observed genesis, therefore its chainID is the genesis commitment
	require.Equal(t, genesisCommitment.ID(), tf.Instance.Storage.Settings.ChainID())

	slot9IssuingTime := tf.SlotTimeProvider().StartTime(9)
	slot10IssuingTime := tf.SlotTimeProvider().StartTime(10)
	slot11IssuingTime := tf.SlotTimeProvider().StartTime(11)
	slot12IssuingTime := tf.SlotTimeProvider().StartTime(12)

	// Slot 9
	tf.BlockDAG.CreateBlock("9.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("8.B")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot9IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 10
	tf.BlockDAG.CreateBlock("10.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("9.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot10IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 11
	tf.BlockDAG.CreateBlock("11.A", models.WithStrongParents(tf.BlockDAG.BlockIDs("10.B")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(slot11IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))
	// Slot 12
	tf.BlockDAG.CreateBlock("12.B", models.WithStrongParents(tf.BlockDAG.BlockIDs("11.A")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(slot12IssuingTime), models.WithCommitment(tf.Instance.Storage.Settings.LatestCommitment()))

	tf.BlockDAG.IssueBlocks("9.A", "10.B", "11.A", "12.B")

	tf.Acceptance.ValidateAcceptedBlocks(map[string]bool{
		"8.B":  true,
		"9.A":  true,
		"10.B": true,
		"11.A": true,
		"12.B": false,
	})

	// We jumped ahead 4 slots, so we evicted rootblocks up to epoch 6, as we confirmed until epoch 9.
	rootBlocks = tf.BlockDAG.Blocks("7.A", "6.B", "8.B", "9.A")
	tf.AssertRootBlocks(rootBlocks)

	// Dump snapshot for latest epoch 9
	snapshotPath := tempDir.Path("snapshot_slot9.bin")
	require.NoError(t, tf.Instance.WriteSnapshot(snapshotPath, 9))

	node2 := mockednetwork.NewNode(t, ed25519.GenerateKeyPair(), testNetwork, "P2", snapshotPath, utxoledger.NewProvider())
	tf2 := node2.EngineTestFramework()

	// We have the same set of rootblocks on the node started from snapshot.
	tf2.AssertRootBlocks(rootBlocks)

	// This node did not observe genesis, therefore its chainID is the snapshot's rootBlocks earliest commitment
	require.Equal(t, lo.PanicOnErr(tf.Instance.Storage.Commitments.Load(1)).ID(), tf2.Instance.Storage.Settings.ChainID())

	require.Equal(t, tf.SlotTimeProvider().GenesisUnixTime(), tf2.SlotTimeProvider().GenesisUnixTime())
	require.Equal(t, tf.ExportBytes(tf.Instance.Storage.Commitments.Export, 9), tf2.ExportBytes(tf2.Instance.Storage.Commitments.Export, 9))
	require.Equal(t, tf.ExportBytes(tf.Instance.Ledger.Export, 9), tf2.ExportBytes(tf2.Instance.Ledger.Export, 9))
	require.Equal(t, tf.ExportBytes(tf.Instance.Notarization.Export, 9), tf2.ExportBytes(tf2.Instance.Notarization.Export, 9))

	node2.Protocol.Shutdown()

	node3 := mockednetwork.NewNodeFromDisk(t, node2.KeyPair, testNetwork, "P3", node2.BaseDir.Path())
	tf3 := node3.EngineTestFramework()

	// Node3 should have the same state as node2.
	tf3.AssertRootBlocks(rootBlocks)
	require.Equal(t, lo.PanicOnErr(tf.Instance.Storage.Commitments.Load(1)).ID(), tf3.Instance.Storage.Settings.ChainID())

	require.Equal(t, tf.SlotTimeProvider().GenesisUnixTime(), tf3.SlotTimeProvider().GenesisUnixTime())
	require.Equal(t, tf.ExportBytes(tf.Instance.Storage.Commitments.Export, 9), tf3.ExportBytes(tf3.Instance.Storage.Commitments.Export, 9))
	require.Equal(t, tf.ExportBytes(tf.Instance.Ledger.Export, 9), tf3.ExportBytes(tf3.Instance.Ledger.Export, 9))
	require.Equal(t, tf.ExportBytes(tf.Instance.Notarization.Export, 9), tf3.ExportBytes(tf3.Instance.Notarization.Export, 9))
}
