package retainer

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
)

// region cachedMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

type cachedMetadata struct {
	BlockDAG      *blockWithTime[*blockdag.Block]
	Booker        *blockWithTime[*booker.Block]
	VirtualVoting *blockWithTime[*virtualvoting.Block]
	Scheduler     *blockWithTime[*scheduler.Block]
	Acceptance    *blockWithTime[*acceptance.Block]

	sync.RWMutex
}

func newCachedMetadata() *cachedMetadata {
	return &cachedMetadata{}
}

func (c *cachedMetadata) setBlock(block any, property any) {
	c.Lock()
	defer c.Unlock()

	property = newBlockWithTime(block)
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
	// StrongChildren           models.BlockIDs `serix:"5"`
	// WeakChildren             models.BlockIDs `serix:"6"`
	// LikedInsteadChildren     models.BlockIDs `serix:"7"`
	SolidTime time.Time `serix:"5"`

	// // booker.Block
	// booked                bool
	// structureDetails      *markers.StructureDetails
	// addedConflictIDs      utxo.TransactionIDs
	// subtractedConflictIDs utxo.TransactionIDs
	// // conflictIDs is a computed property at the time a block is booked.
	// conflictIDs           utxo.TransactionIDs
}

func newBlockMetadata(cm *cachedMetadata) (b *BlockMetadata) {
	if cm == nil {
		b = model.NewStorable[models.BlockID, BlockMetadata](&blockMetadataModel{})
		b.SetID(models.EmptyBlockID)
		return b
	}

	cm.RLock()
	defer cm.RUnlock()

	b = model.NewStorable[models.BlockID, BlockMetadata](&blockMetadataModel{
		Missing:  cm.BlockDAG.Block.IsMissing(),
		Solid:    cm.BlockDAG.Block.IsSolid(),
		Invalid:  cm.BlockDAG.Block.IsInvalid(),
		Orphaned: cm.BlockDAG.Block.IsOrphaned(),
		// OrphanedBlocksInPastCone: cm.BlockDAG.Block.OrphanedBlocksInPastCone().Clone(),
		// StrongChildren:           blocksToBlockIDs(cm.BlockDAG.Block.StrongChildren()),
		// WeakChildren:             blocksToBlockIDs(cm.BlockDAG.Block.WeakChildren()),
		// LikedInsteadChildren:     blocksToBlockIDs(cm.BlockDAG.Block.LikedInsteadChildren()),
		SolidTime: cm.BlockDAG.Time,
	})
	b.SetID(cm.BlockDAG.Block.ID())

	return b
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func blocksToBlockIDs(blocks []*blockdag.Block) []string {
	return lo.Map(blocks, func(block *blockdag.Block) string { return block.ID().Base58() })
}
