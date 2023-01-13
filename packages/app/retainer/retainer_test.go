package retainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/serix"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestRetainer_BlockMetadata_Serialization(t *testing.T) {
	meta := createBlockMetadata()

	serializedBytes, err := meta.Bytes()
	require.NoError(t, err)

	metaDeserialized := newBlockMetadata(nil)
	decodedBytes, err := metaDeserialized.FromBytes(serializedBytes)
	require.NoError(t, err)
	require.Equal(t, len(serializedBytes), decodedBytes)

	validateDeserialized(t, meta, metaDeserialized)
}

func TestRetainer_BlockMetadata_JSON(t *testing.T) {
	meta := createBlockMetadata()
	out, err := serix.DefaultAPI.JSONEncode(context.Background(), meta.M)
	require.NoError(t, err)
	printPrettyJSON(t, out)

	metaDeserialized := newBlockMetadata(nil)
	err = serix.DefaultAPI.JSONDecode(context.Background(), out, &metaDeserialized.M)
	require.NoError(t, err)
	validateDeserialized(t, meta, metaDeserialized)
}

func TestRetainer_BlockMetadata_JSON_optional(t *testing.T) {
	meta := createBlockMetadata()
	meta.M.StructureDetails = nil
	out, err := serix.DefaultAPI.JSONEncode(context.Background(), meta.M)
	require.NoError(t, err)
	printPrettyJSON(t, out)

	metaDeserialized := newBlockMetadata(nil)
	err = serix.DefaultAPI.JSONDecode(context.Background(), out, &metaDeserialized.M)
	require.NoError(t, err)
	validateDeserialized(t, meta, metaDeserialized)
}

func TestRetainer_BlockMetadata_NonEvicted(t *testing.T) {
	protocolTF := protocol.NewTestFramework(t)
	protocolTF.Protocol.Run()

	retainer := NewRetainer(protocolTF.Protocol, database.NewManager(0))
	t.Cleanup(func() {
		retainer.Shutdown()
	})

	tangleTF := tangle.NewTestFramework(t, tangle.WithTangle(protocolTF.Protocol.Engine().Tangle))
	b := tangleTF.CreateBlock("A")
	tangleTF.IssueBlocks("A").WaitUntilAllTasksProcessed()
	block, exists := protocolTF.Protocol.CongestionControl.Block(b.ID())
	require.True(t, exists)
	var meta *BlockMetadata
	require.Eventuallyf(t, func() (exists bool) {
		meta, exists = retainer.BlockMetadata(block.ID())

		return exists && meta.M.Scheduled
	}, 5*time.Second, 10*time.Millisecond, "block metadata should be available")

	require.Equal(t, meta.M.Missing, block.IsMissing())
	require.Equal(t, meta.M.Solid, block.IsSolid())
	require.Equal(t, meta.M.Invalid, block.IsInvalid())
	require.Equal(t, meta.M.Orphaned, block.IsOrphaned())
	require.Equal(t, meta.M.StrongChildren, blocksToBlockIDs(block.StrongChildren()))
	require.Equal(t, meta.M.WeakChildren, blocksToBlockIDs(block.WeakChildren()))
	require.Equal(t, meta.M.LikedInsteadChildren, blocksToBlockIDs(block.LikedInsteadChildren()))
	require.Equal(t, meta.M.Booked, block.IsBooked())
	require.EqualValues(t, meta.M.StructureDetails.IsPastMarker, block.StructureDetails().IsPastMarker())
	require.EqualValues(t, meta.M.StructureDetails.Rank, block.StructureDetails().Rank())
	require.EqualValues(t, meta.M.StructureDetails.PastMarkerGap, block.StructureDetails().PastMarkerGap())

	pastMarkers := markers.NewMarkers()
	for sequenceID, index := range meta.M.StructureDetails.PastMarkers {
		pastMarkers.Set(sequenceID, index)
	}
	require.EqualValues(t, pastMarkers, block.StructureDetails().PastMarkers())
	require.Equal(t, meta.M.AddedConflictIDs, block.AddedConflictIDs())
	require.Equal(t, meta.M.SubtractedConflictIDs, block.SubtractedConflictIDs())
	require.Equal(t, meta.M.ConflictIDs, protocolTF.Protocol.Engine().Tangle.BlockConflicts(block.Block.Block))

	require.Equal(t, meta.M.Tracked, true)
	require.Equal(t, meta.M.SubjectivelyInvalid, block.IsSubjectivelyInvalid())
	require.Equal(t, meta.M.Scheduled, block.IsScheduled())
	require.Equal(t, meta.M.Skipped, block.IsSkipped())
	require.Equal(t, meta.M.Dropped, block.IsDropped())
	require.Equal(t, meta.M.Accepted, false)
}

func TestRetainer_BlockMetadata_Evicted(t *testing.T) {
	protocolTF := protocol.NewTestFramework(t)
	protocolTF.Protocol.Run()

	retainer := NewRetainer(protocolTF.Protocol, database.NewManager(0))
	t.Cleanup(func() {
		retainer.Shutdown()
	})

	tangleTF := tangle.NewTestFramework(t, tangle.WithTangle(protocolTF.Protocol.Engine().Tangle))
	b := tangleTF.CreateBlock("A", models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0).Add(70*time.Second)))
	tangleTF.IssueBlocks("A").WaitUntilAllTasksProcessed()
	block, exists := protocolTF.Protocol.CongestionControl.Block(b.ID())
	require.True(t, exists)
	protocolTF.Protocol.Engine().EvictionState.EvictUntil(b.ID().EpochIndex + 1)
	protocolTF.WaitUntilAllTasksProcessed()
	retainer.WorkerPool().PendingTasksCounter.WaitIsZero()

	meta, exists := retainer.BlockMetadata(block.ID())
	require.True(t, exists)

	require.Equal(t, meta.M.Missing, block.IsMissing())
	require.Equal(t, meta.M.Solid, block.IsSolid())
	require.Equal(t, meta.M.Invalid, block.IsInvalid())
	require.Equal(t, meta.M.Orphaned, block.IsOrphaned())
	require.Equal(t, meta.M.StrongChildren, blocksToBlockIDs(block.StrongChildren()))
	require.Equal(t, meta.M.WeakChildren, blocksToBlockIDs(block.WeakChildren()))
	require.Equal(t, meta.M.LikedInsteadChildren, blocksToBlockIDs(block.LikedInsteadChildren()))
	require.Equal(t, meta.M.Booked, block.IsBooked())
	if meta.M.StructureDetails != nil {
		require.EqualValues(t, meta.M.StructureDetails.IsPastMarker, block.StructureDetails().IsPastMarker())
		require.EqualValues(t, meta.M.StructureDetails.Rank, block.StructureDetails().Rank())
		require.EqualValues(t, meta.M.StructureDetails.PastMarkerGap, block.StructureDetails().PastMarkerGap())
	}

	pastMarkers := markers.NewMarkers()
	for sequenceID, index := range meta.M.StructureDetails.PastMarkers {
		pastMarkers.Set(sequenceID, index)
	}
	require.EqualValues(t, pastMarkers, block.StructureDetails().PastMarkers())
	require.Equal(t, meta.M.AddedConflictIDs, block.AddedConflictIDs())
	require.Equal(t, meta.M.SubtractedConflictIDs, block.SubtractedConflictIDs())
	require.Equal(t, meta.M.ConflictIDs, protocolTF.Protocol.Engine().Tangle.BlockConflicts(block.Block.Block))
	require.Equal(t, meta.M.Tracked, true)
	require.Equal(t, meta.M.SubjectivelyInvalid, block.IsSubjectivelyInvalid())
	// You cannot really test this as the scheduler might have scheduled the block after its metadata was retained.
	// require.Equal(t, meta.M.Scheduled, block.IsScheduled())
	require.Equal(t, meta.M.Skipped, block.IsSkipped())
	require.Equal(t, meta.M.Dropped, block.IsDropped())
	require.Equal(t, meta.M.Accepted, false)
}

func validateDeserialized(t *testing.T, meta *BlockMetadata, metaDeserialized *BlockMetadata) {
	require.Equal(t, meta.M.Missing, metaDeserialized.M.Missing)
	require.Equal(t, meta.M.Solid, metaDeserialized.M.Solid)
	require.Equal(t, meta.M.Invalid, metaDeserialized.M.Invalid)
	require.Equal(t, meta.M.Orphaned, metaDeserialized.M.Orphaned)
	require.Equal(t, meta.M.StrongChildren, metaDeserialized.M.StrongChildren)
	require.Equal(t, meta.M.WeakChildren, metaDeserialized.M.WeakChildren)
	require.Equal(t, meta.M.LikedInsteadChildren, metaDeserialized.M.LikedInsteadChildren)
	require.Equal(t, meta.M.SolidTime.Unix(), metaDeserialized.M.SolidTime.Unix())
	require.Equal(t, meta.M.Booked, metaDeserialized.M.Booked)
	require.EqualValues(t, meta.M.StructureDetails, metaDeserialized.M.StructureDetails)
	// TODO: implement JSON serialization for AdvancedSet or OrderedMap
	// require.Equal(t, meta.M.AddedConflictIDs, metaDeserialized.M.AddedConflictIDs)
	// require.Equal(t, meta.M.SubtractedConflictIDs, metaDeserialized.M.SubtractedConflictIDs)
	// require.Equal(t, meta.M.ConflictIDs, metaDeserialized.M.ConflictIDs)
	require.Equal(t, meta.M.BookedTime.Unix(), metaDeserialized.M.BookedTime.Unix())
	require.Equal(t, meta.M.Tracked, metaDeserialized.M.Tracked)
	require.Equal(t, meta.M.SubjectivelyInvalid, metaDeserialized.M.SubjectivelyInvalid)
	require.Equal(t, meta.M.TrackedTime.Unix(), metaDeserialized.M.TrackedTime.Unix())
	require.Equal(t, meta.M.Scheduled, metaDeserialized.M.Scheduled)
	require.Equal(t, meta.M.Skipped, metaDeserialized.M.Skipped)
	require.Equal(t, meta.M.Dropped, metaDeserialized.M.Dropped)
	require.Equal(t, meta.M.SchedulerTime.Unix(), metaDeserialized.M.SchedulerTime.Unix())
	require.Equal(t, meta.M.Accepted, metaDeserialized.M.Accepted)
	require.Equal(t, meta.M.AcceptedTime.Unix(), metaDeserialized.M.AcceptedTime.Unix())
}

func createBlockMetadata() *BlockMetadata {
	var blockID0, blockID1, blockID2 models.BlockID
	_ = blockID0.FromRandomness()
	_ = blockID1.FromRandomness()
	_ = blockID2.FromRandomness()

	meta := newBlockMetadata(nil)
	meta.SetID(blockID0)
	meta.M.Missing = false
	meta.M.Solid = true
	meta.M.Invalid = false
	meta.M.Orphaned = true
	meta.M.StrongChildren = make(models.BlockIDs)
	meta.M.StrongChildren.Add(blockID2)
	meta.M.WeakChildren = make(models.BlockIDs)
	meta.M.WeakChildren.Add(blockID2)
	meta.M.LikedInsteadChildren = make(models.BlockIDs)
	meta.M.LikedInsteadChildren.Add(blockID2)
	meta.M.SolidTime = time.Now()

	meta.M.Booked = true
	meta.M.StructureDetails = &structureDetails{
		Rank:          4,
		PastMarkerGap: 3,
		IsPastMarker:  true,
		PastMarkers:   map[markers.SequenceID]markers.Index{markers.SequenceID(5): markers.Index(1)},
	}
	meta.M.AddedConflictIDs = utxo.NewTransactionIDs(utxo.EmptyTransactionID, utxo.NewTransactionID([]byte("test")))
	meta.M.SubtractedConflictIDs = utxo.NewTransactionIDs(utxo.NewTransactionID([]byte("test1")), utxo.NewTransactionID([]byte("test2")))
	meta.M.ConflictIDs = utxo.NewTransactionIDs(utxo.NewTransactionID([]byte("test1")), utxo.NewTransactionID([]byte("test2")))
	meta.M.BookedTime = time.Now()
	meta.M.Tracked = true
	meta.M.SubjectivelyInvalid = true
	meta.M.TrackedTime = time.Now()
	meta.M.Scheduled = true
	meta.M.Skipped = false
	meta.M.Dropped = false
	meta.M.SchedulerTime = time.Now()
	meta.M.Accepted = true
	meta.M.AcceptedTime = time.Now()
	return meta
}

func printPrettyJSON(t *testing.T, b []byte) {
	var prettyJSON bytes.Buffer
	require.NoError(t, json.Indent(&prettyJSON, b, "", "    "))
	fmt.Println(prettyJSON.String())
}
