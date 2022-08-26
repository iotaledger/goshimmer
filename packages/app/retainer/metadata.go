package retainer

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"

	"github.com/iotaledger/goshimmer/packages/core/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/virtualvoting"
)

type cachedMetadata struct {
	BlockDAG blockWithTime[*blockdag.Block]
	Booker   blockWithTime[*booker.Block]
	OTV      blockWithTime[*virtualvoting.Block]

	m sync.RWMutex
}

type blockWithTime[BlockType any] struct {
	Block BlockType
	Time  time.Time
}

type BlockMetadata struct {
	BlockID models.BlockID

	// blockdag.Block
	Missing                  bool     `serix:"0"`
	Solid                    bool     `serix:"1"`
	Invalid                  bool     `serix:"2"`
	Orphaned                 bool     `serix:"3"`
	OrphanedBlocksInPastCone []string `serix:"4"`
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
		return nil
	}

	b = &BlockMetadata{
		BlockID:  cm.BlockDAG.Block.ID(),
		Missing:  cm.BlockDAG.Block.IsMissing(),
		Solid:    cm.BlockDAG.Block.IsSolid(),
		Invalid:  cm.BlockDAG.Block.IsInvalid(),
		Orphaned: cm.BlockDAG.Block.IsOrphaned(),
		// OrphanedBlocksInPastCone: cm.BlockDAG.Block.OrphanedBlocksInPastCone().Clone(),
		// StrongChildren:           blocksToBlockIDs(cm.BlockDAG.Block.StrongChildren()),
		// WeakChildren:             blocksToBlockIDs(cm.BlockDAG.Block.WeakChildren()),
		// LikedInsteadChildren:     blocksToBlockIDs(cm.BlockDAG.Block.LikedInsteadChildren()),
		SolidTime: cm.BlockDAG.Time,
	}

	return b
}

func blocksToBlockIDs(blocks []*blockdag.Block) []string {
	return lo.Map(blocks, func(block *blockdag.Block) string { return block.ID().Base58() })
}
