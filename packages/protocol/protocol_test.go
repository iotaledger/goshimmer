package protocol

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-playground/assert/v2"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/stretchr/testify/require"
)

func TestProtocol(t *testing.T) {
	debug.SetEnabled(true)

	testNetwork := network.NewMockedNetwork()

	endpoint1 := testNetwork.Join(identity.GenerateIdentity().ID())
	diskUtil1 := diskutil.New(t.TempDir())

	snapshotcreator.CreateSnapshot(DatabaseVersion, diskUtil1.Path("snapshot.bin"), 100, make([]byte, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	})

	protocol1 := New(endpoint1, WithBaseDirectory(diskUtil1.Path()), WithSnapshotPath(diskUtil1.Path("snapshot.bin")))
	protocol1.Run()

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
	diskUtil2 := diskutil.New(t.TempDir())

	snapshotcreator.CreateSnapshot(DatabaseVersion, diskUtil2.Path("snapshot.bin"), 100, make([]byte, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	})

	protocol2 := New(endpoint2, WithBaseDirectory(diskUtil2.Path()), WithSnapshotPath(diskUtil2.Path("snapshot.bin")))
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
	_ = NewEngineTestFramework(t, WithEngine(protocol2.Engine()))

	tf1.Tangle.CreateBlock("A", models.WithStrongParents(tf1.Tangle.BlockIDs("Genesis")))
	tf1.Tangle.IssueBlocks("A")

	time.Sleep(4 * time.Second)

	event.Loop.PendingTasksCounter.WaitIsZero()
}

func TestEngine_WriteSnapshot(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	epoch.GenesisTime = time.Now().Unix() - epoch.Duration*10

	fmt.Println("> GenesisTime", time.Unix(epoch.GenesisTime, 0))

	tf := NewEngineTestFramework(t)
	tempDisk := diskutil.New(t.TempDir())

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

	snapshotcreator.CreateSnapshot(DatabaseVersion, tempDisk.Path("genesis_snapshot.bin"), 1, make([]byte, 32), identitiesWeights)

	tf.Engine.Initialize(tempDisk.Path("genesis_snapshot.bin"))

	fmt.Println("+++", lo.PanicOnErr(tf.Engine.SybilProtection.Weights().Map()))

	tf.Engine.Clock.Events.AcceptanceTimeUpdated.Hook(event.NewClosure(func(event *clock.TimeUpdate) {
		fmt.Println("> AcceptanceTimeUpdated", event.NewTime)
	}))

	acceptedBlocks := make(map[string]bool)

	epoch1IssuingTime := time.Unix(epoch.GenesisTime+1, 0)

	// Blocks in epoch 1
	tf.Tangle.CreateBlock("A.1", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithIssuer(identitiesMap["A"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.CreateBlock("B.2", models.WithStrongParents(tf.Tangle.BlockIDs("A.1")), models.WithIssuer(identitiesMap["B"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.CreateBlock("C.3", models.WithStrongParents(tf.Tangle.BlockIDs("B.2")), models.WithIssuer(identitiesMap["C"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.CreateBlock("D.4", models.WithStrongParents(tf.Tangle.BlockIDs("C.3")), models.WithIssuer(identitiesMap["D"]), models.WithIssuingTime(epoch1IssuingTime))
	tf.Tangle.IssueBlocks("A.1", "B.2", "C.3", "D.4").WaitUntilAllTasksProcessed()

	tf.Acceptance.AssertBlockTracked(4)

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"A.1": true,
		"B.2": true,
		"C.3": false,
		"D.4": false,
	}))

	// Block in epoch 11
	tf.Tangle.CreateBlock("A.5", models.WithStrongParents(tf.Tangle.BlockIDs("D.4")), models.WithIssuer(identitiesMap["A"]))
	tf.Tangle.IssueBlocks("A.5").WaitUntilAllTasksProcessed()

	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"C.3": true,
		"A.5": false,
	}))

	assert.Equal(t, epoch.IndexFromTime(tf.Tangle.Block("A.5").IssuingTime()), epoch.Index(11))

	// Time hasn't advanced past epoch 1
	assert.Equal(t, tf.Engine.Storage.Settings.LatestCommitment().Index(), epoch.Index(0))

	tf.Tangle.CreateBlock("B.6", models.WithStrongParents(tf.Tangle.BlockIDs("A.5")), models.WithIssuer(identitiesMap["B"]))
	tf.Tangle.CreateBlock("C.7", models.WithStrongParents(tf.Tangle.BlockIDs("B.6")), models.WithIssuer(identitiesMap["C"]))
	tf.Tangle.IssueBlocks("B.6", "C.7").WaitUntilAllTasksProcessed()

	// Some blocks got evicted and we have to restart evaluating with a new map
	acceptedBlocks = make(map[string]bool)
	tf.Acceptance.ValidateAcceptedBlocks(lo.MergeMaps(acceptedBlocks, map[string]bool{
		"A.5": true,
	}))

	// Time has advanced to epoch 10 because of A.5, rendering 10 - MinimumCommittableAge(6) = 4 epoch committable
	assert.Equal(t, tf.Engine.Storage.Settings.LatestCommitment().Index(), epoch.Index(4))

	tf.Engine.WriteSnapshot(tempDisk.Path("snapshot_epoch4.bin"))

	tf2 := NewEngineTestFramework(t)

	tf2.Engine.Initialize(tempDisk.Path("snapshot_epoch4.bin"))

	// Settings
	// The ChainID of the new engine corresponds to the target epoch of the imported snapshot.
	assert.Equal(t, lo.PanicOnErr(tf.Engine.Storage.Commitments.Load(4)).ID(), tf2.Engine.Storage.Settings.ChainID())
	assert.Equal(t, tf.Engine.Storage.Settings.LatestCommitment(), tf2.Engine.Storage.Settings.LatestCommitment())
	assert.Equal(t, tf.Engine.Storage.Settings.LatestConfirmedEpoch(), tf2.Engine.Storage.Settings.LatestConfirmedEpoch())
	assert.Equal(t, tf.Engine.Storage.Settings.LatestStateMutationEpoch(), tf2.Engine.Storage.Settings.LatestStateMutationEpoch())

	// LedgerState
	assert.Equal(t, tf.Engine.LedgerState.UnspentOutputs.IDs.Size(), tf2.Engine.LedgerState.UnspentOutputs.IDs.Size())
	assert.Equal(t, tf.Engine.LedgerState.UnspentOutputs.IDs.Root(), tf2.Engine.LedgerState.UnspentOutputs.IDs.Root())
	tf.Engine.LedgerState.UnspentOutputs.IDs.Stream(func(outputID utxo.OutputID) bool {
		require.True(t, tf2.Engine.LedgerState.UnspentOutputs.IDs.Has(outputID))
		return true
	})

	// SybilProtection
	assert.Equal(t, lo.PanicOnErr(tf.Engine.SybilProtection.Weights().Map()), lo.PanicOnErr(tf2.Engine.SybilProtection.Weights().Map()))
	assert.Equal(t, tf.Engine.SybilProtection.Weights().TotalWeight(), tf2.Engine.SybilProtection.Weights().TotalWeight())
	assert.Equal(t, tf.Engine.SybilProtection.Weights().Root(), tf2.Engine.SybilProtection.Weights().Root())

	for epochIndex := epoch.Index(0); epochIndex <= 4; epochIndex++ {
		originalCommitment, err := tf.Engine.Storage.Commitments.Load(epochIndex)
		require.NoError(t, err)
		importedCommitment, err := tf2.Engine.Storage.Commitments.Load(epochIndex)
		require.NoError(t, err)

		assert.Equal(t, originalCommitment, importedCommitment)
	}
}
