package retainer

import (
	"context"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region cachedMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

type cachedMetadata struct {
	BlockDAG *blockWithTime[*blockdag.Block]
	Booker   *blockWithTime[*booker.Block]
	// calculated property
	ConflictIDs   utxo.TransactionIDs
	VirtualVoting *blockWithTime[*virtualvoting.Block]
	Scheduler     *blockWithTime[*scheduler.Block]
	Acceptance    *blockWithTime[*acceptance.Block]

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

func (c *cachedMetadata) setVirtualVotingBlock(block *virtualvoting.Block) {
	c.Lock()
	defer c.Unlock()
	c.VirtualVoting = newBlockWithTime(block)
}

func (c *cachedMetadata) setAcceptanceBlock(block *acceptance.Block) {
	c.Lock()
	defer c.Unlock()
	c.Acceptance = newBlockWithTime(block)
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
	// blockdag.Block
	Missing                  bool            `serix:"0"`
	Solid                    bool            `serix:"1"`
	Invalid                  bool            `serix:"2"`
	Orphaned                 bool            `serix:"3"`
	OrphanedBlocksInPastCone models.BlockIDs `serix:"4"`
	StrongChildren           models.BlockIDs `serix:"5"`
	WeakChildren             models.BlockIDs `serix:"6"`
	LikedInsteadChildren     models.BlockIDs `serix:"7"`
	SolidTime                time.Time       `serix:"8"`

	// booker.Block
	Booked                bool                `serix:"9"`
	StructureDetails      *structureDetails   `serix:"10"`
	AddedConflictIDs      utxo.TransactionIDs `serix:"11"`
	SubtractedConflictIDs utxo.TransactionIDs `serix:"12"`
	// conflictIDs is a computed property at the time a block is booked.
	ConflictIDs utxo.TransactionIDs `serix:"13"`
	BookedTime  time.Time           `serix:"14"`

	// virtualvoting.Block
	Tracked             bool      `serix:"15"`
	SubjectivelyInvalid bool      `serix:"16"`
	TrackedTime         time.Time `serix:"17"`

	// scheduler.Block
	Scheduled     bool      `serix:"18"`
	Skipped       bool      `serix:"19"`
	Dropped       bool      `serix:"20"`
	SchedulerTime time.Time `serix:"21"`

	// acceptance.Block
	Accepted     bool      `serix:"22"`
	AcceptedTime time.Time `serix:"23"`
}

func (b *BlockMetadata) Encode() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), b.M)
}

func (b *BlockMetadata) Decode(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, &b.M)
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
		copyFromBlockDAGBlock(cm.BlockDAG, b)
		b.SetID(cm.BlockDAG.Block.ID())
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

	return b
}

func copyFromBlockDAGBlock(blockWithTime *blockWithTime[*blockdag.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block
	blockMetadata.M.Missing = block.IsMissing()
	blockMetadata.M.Solid = block.IsSolid()
	blockMetadata.M.Invalid = block.IsInvalid()
	blockMetadata.M.Orphaned = block.IsOrphaned()
	blockMetadata.M.OrphanedBlocksInPastCone = block.OrphanedBlocksInPastCone().Clone()
	blockMetadata.M.StrongChildren = blocksToBlockIDs(block.StrongChildren())
	blockMetadata.M.WeakChildren = blocksToBlockIDs(block.WeakChildren())
	blockMetadata.M.LikedInsteadChildren = blocksToBlockIDs(block.LikedInsteadChildren())
	blockMetadata.M.SolidTime = blockWithTime.Time
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

func copyFromVirtualVotingBlock(blockWithTime *blockWithTime[*virtualvoting.Block], blockMetadata *BlockMetadata) {
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

func copyFromAcceptanceBlock(blockWithTime *blockWithTime[*acceptance.Block], blockMetadata *BlockMetadata) {
	block := blockWithTime.Block
	blockMetadata.M.Accepted = block.IsAccepted()
	blockMetadata.M.SchedulerTime = blockWithTime.Time
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
