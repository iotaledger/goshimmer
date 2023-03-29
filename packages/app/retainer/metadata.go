package retainer

import (
	"context"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/objectstorage/generic/model"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

// region cachedMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

type cachedMetadata struct {
	BlockDAG *blockWithTime[*blockdag.Block]
	Booker   *blockWithTime[*booker.Block]
	// calculated property
	ConflictIDs   utxo.TransactionIDs
	VirtualVoting *blockWithTime[*booker.Block]
	Scheduler     *blockWithTime[*scheduler.Block]
	Acceptance    *blockWithTime[*blockgadget.Block]
	Confirmation  *blockWithTime[*blockgadget.Block]

	sync.RWMutex
}

func newCachedMetadata() *cachedMetadata {
	return &cachedMetadata{}
}

func (c *cachedMetadata) setBlockDAGBlock(block *blockdag.Block) {
	c.Lock()
	defer c.Unlock()
	c.BlockDAG = newBlockWithTime(block)
}

func (c *cachedMetadata) setBookerBlock(block *booker.Block) {
	c.Lock()
	defer c.Unlock()
	c.Booker = newBlockWithTime(block)
}

func (c *cachedMetadata) setVirtualVotingBlock(block *booker.Block) {
	c.Lock()
	defer c.Unlock()
	c.VirtualVoting = newBlockWithTime(block)
}

func (c *cachedMetadata) setAcceptanceBlock(block *blockgadget.Block) {
	c.Lock()
	defer c.Unlock()
	c.Acceptance = newBlockWithTime(block)
}

func (c *cachedMetadata) setConfirmationBlock(block *blockgadget.Block) {
	c.Lock()
	defer c.Unlock()
	c.Confirmation = newBlockWithTime(block)
}

func (c *cachedMetadata) setSchedulerBlock(block *scheduler.Block) {
	c.Lock()
	defer c.Unlock()
	c.Scheduler = newBlockWithTime(block)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region blockWithTime ////////////////////////////////////////////////////////////////////////////////////////////////

type blockWithTime[BlockType any] struct {
	Block BlockType
	Time  time.Time
}

func newBlockWithTime[BlockType any](block BlockType) *blockWithTime[BlockType] {
	return &blockWithTime[BlockType]{
		Block: block,
		Time:  time.Now(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region cachedMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// BlockMetadata stores block metadata generated during processing of the block.
type BlockMetadata struct {
	model.Storable[models.BlockID, BlockMetadata, *BlockMetadata, blockMetadataModel] `serix:"0"`
}

type blockMetadataModel struct {
	ID models.BlockID `serix:"0"`

	// blockdag.Block
	Missing  bool `serix:"1"`
	Solid    bool `serix:"2"`
	Invalid  bool `serix:"3"`
	Orphaned bool `serix:"4"`
	// TODO: children need to be stored separately due to potentially unbounded size
	StrongChildren models.BlockIDs `serix:"6,lengthPrefixType=uint32"`
	// TODO: children need to be stored separately due to potentially unbounded size
	WeakChildren models.BlockIDs `serix:"7,lengthPrefixType=uint32"`
	// TODO: children need to be stored separately due to potentially unbounded size
	LikedInsteadChildren models.BlockIDs `serix:"8,lengthPrefixType=uint32"`
	SolidTime            time.Time       `serix:"9"`

	// booker.Block
	Booked           bool              `serix:"10"`
	StructureDetails *structureDetails `serix:"11,optional"`
	// TODO: conflicts need to be stored separately due to potentially unbounded size
	AddedConflictIDs utxo.TransactionIDs `serix:"12,optional"`
	// TODO: conflicts need to be stored separately due to potentially unbounded size
	SubtractedConflictIDs utxo.TransactionIDs `serix:"13,optional"`
	// TODO: conflicts need to be stored separately due to potentially unbounded size
	// conflictIDs is a computed property at the time a block is booked.
	ConflictIDs utxo.TransactionIDs `serix:"14,optional"`
	BookedTime  time.Time           `serix:"15"`

	// virtualvoting.Block
	Tracked             bool      `serix:"16"`
	SubjectivelyInvalid bool      `serix:"17"`
	TrackedTime         time.Time `serix:"18"`

	// scheduler.Block
	Scheduled     bool      `serix:"19"`
	Skipped       bool      `serix:"20"`
	Dropped       bool      `serix:"21"`
	SchedulerTime time.Time `serix:"22"`

	// blockgadget.Block
	Accepted     bool      `serix:"23"`
	AcceptedTime time.Time `serix:"24"`

	// confirmation.Block
	Confirmed           bool      `serix:"25"`
	ConfirmedTime       time.Time `serix:"26"`
	ConfirmedBySlot     bool      `serix:"27"`
	ConfirmedBySlotTime time.Time `serix:"28"`

	Block *models.Block `serix:"29,optional"`
}

// NewBlockMetadata creates a new BlockMetadata instance. It does not set the ID, as it is not known at this point.
func NewBlockMetadata() (b *BlockMetadata) {
	return model.NewStorable[models.BlockID, BlockMetadata](&blockMetadataModel{})
}

func (b *BlockMetadata) Encode() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), b.M)
}

func (b *BlockMetadata) Decode(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, &b.M)
}

func (b *BlockMetadata) MarshalJSON() ([]byte, error) {
	return serix.DefaultAPI.JSONEncode(context.Background(), b.M)
}

func (b *BlockMetadata) UnmarshalJSON(bytes []byte) error {
	return serix.DefaultAPI.JSONDecode(context.Background(), bytes, &b.M)
}

func newBlockMetadata(cm *cachedMetadata) (b *BlockMetadata) {
	if cm == nil {
		b = model.NewStorable[models.BlockID, BlockMetadata](&blockMetadataModel{})
		b.SetID(models.EmptyBlockID)
		return b
	}

	cm.RLock()
	defer cm.RUnlock()

	b = model.NewStorable[models.BlockID, BlockMetadata](&blockMetadataModel{})

	if cm.BlockDAG != nil {
		b.M.ID = cm.BlockDAG.Block.ID()
		b.SetID(cm.BlockDAG.Block.ID())
		copyFromBlockDAGBlock(cm.BlockDAG, b)
	}

	if cm.Booker != nil {
		copyFromBookerBlock(cm.Booker, b)
		b.M.ConflictIDs = cm.ConflictIDs
	}

	if cm.VirtualVoting != nil {
		copyFromVirtualVotingBlock(cm.VirtualVoting, b)
	}

	if cm.Scheduler != nil {
		copyFromSchedulerBlock(cm.Scheduler, b)
	}

	if cm.Acceptance != nil {
		copyFromAcceptanceBlock(cm.Acceptance, b)
	}

	if cm.Confirmation != nil {
		copyFromConfirmedBlock(cm.Confirmation, b)
	}
	return b
}

func copyFromBlockDAGBlock(blockWithTime *blockWithTime[*blockdag.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block

	blockMetadata.M.Missing = block.IsMissing()
	blockMetadata.M.Solid = block.IsSolid()
	blockMetadata.M.Invalid = block.IsInvalid()
	blockMetadata.M.Orphaned = block.IsOrphaned()
	blockMetadata.M.StrongChildren = blocksToBlockIDs(block.StrongChildren())
	blockMetadata.M.WeakChildren = blocksToBlockIDs(block.WeakChildren())
	blockMetadata.M.LikedInsteadChildren = blocksToBlockIDs(block.LikedInsteadChildren())
	blockMetadata.M.SolidTime = blockWithTime.Time
	blockMetadata.M.Block = blockWithTime.Block.ModelsBlock
}

func copyFromBookerBlock(blockWithTime *blockWithTime[*booker.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block

	blockMetadata.M.Booked = block.IsBooked()
	if structDetails := block.StructureDetails(); structDetails != nil {
		pastMarkers := make(map[markers.SequenceID]markers.Index)
		structDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			pastMarkers[sequenceID] = index
			return true
		})

		blockMetadata.M.StructureDetails = &structureDetails{
			Rank:          structDetails.Rank(),
			PastMarkerGap: structDetails.PastMarkerGap(),
			IsPastMarker:  structDetails.IsPastMarker(),
			PastMarkers:   pastMarkers,
		}
	}
	blockMetadata.M.AddedConflictIDs = block.AddedConflictIDs().Clone()
	blockMetadata.M.SubtractedConflictIDs = block.SubtractedConflictIDs().Clone()
	blockMetadata.M.BookedTime = blockWithTime.Time
}

func copyFromVirtualVotingBlock(blockWithTime *blockWithTime[*booker.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block

	blockMetadata.M.Tracked = true
	blockMetadata.M.SubjectivelyInvalid = block.IsSubjectivelyInvalid()
	blockMetadata.M.TrackedTime = blockWithTime.Time
}

func copyFromSchedulerBlock(blockWithTime *blockWithTime[*scheduler.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block

	blockMetadata.M.Scheduled = block.IsScheduled()
	blockMetadata.M.Skipped = block.IsSkipped()
	blockMetadata.M.Dropped = block.IsDropped()
	blockMetadata.M.SchedulerTime = blockWithTime.Time
}

func copyFromAcceptanceBlock(blockWithTime *blockWithTime[*blockgadget.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block

	blockMetadata.M.Accepted = block.IsAccepted()
	blockMetadata.M.AcceptedTime = blockWithTime.Time
}

func copyFromConfirmedBlock(blockWithTime *blockWithTime[*blockgadget.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block

	blockMetadata.M.Confirmed = block.IsConfirmed()
	blockMetadata.M.ConfirmedTime = blockWithTime.Time
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region structureDetails ///////////////////////////////////////////////////////////////////////////////////////////////

type structureDetails struct {
	Rank          uint64                               `serix:"1"`
	PastMarkerGap uint64                               `serix:"2"`
	IsPastMarker  bool                                 `serix:"3"`
	PastMarkers   map[markers.SequenceID]markers.Index `serix:"4,lengthPrefixType=byte"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func blocksToBlockIDs(blocks []*blockdag.Block) models.BlockIDs {
	return models.NewBlockIDs(lo.Map(blocks, func(block *blockdag.Block) models.BlockID { return block.ID() })...)
}
