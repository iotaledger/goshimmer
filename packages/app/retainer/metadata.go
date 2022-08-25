package retainer

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"

	"github.com/iotaledger/goshimmer/packages/core/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/otv"
)

type cachedMetadata struct {
	BlockDAG blockWithTime[*blockdag.Block]
	Booker   blockWithTime[*booker.Block]
	OTV      blockWithTime[*otv.Block]

	m sync.RWMutex
}

type blockWithTime[BlockType any] struct {
	Block BlockType
	Time  time.Time
}

// TODO: make BlockMetadata work with serix serialization and JSON
type BlockMetadata struct {
	// blockdag.Block
	missing                  bool
	solid                    bool
	invalid                  bool
	orphaned                 bool
	orphanedBlocksInPastCone models.BlockIDs
	strongChildren           models.BlockIDs
	weakChildren             models.BlockIDs
	likedInsteadChildren     models.BlockIDs
	solidTime                time.Time

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
		missing:                  cm.BlockDAG.Block.IsMissing(),
		solid:                    cm.BlockDAG.Block.IsSolid(),
		invalid:                  cm.BlockDAG.Block.IsInvalid(),
		orphaned:                 cm.BlockDAG.Block.IsOrphaned(),
		orphanedBlocksInPastCone: cm.BlockDAG.Block.OrphanedBlocksInPastCone().Clone(),
		strongChildren:           blocksToBlockIDs(cm.BlockDAG.Block.StrongChildren()),
		weakChildren:             blocksToBlockIDs(cm.BlockDAG.Block.WeakChildren()),
		likedInsteadChildren:     blocksToBlockIDs(cm.BlockDAG.Block.LikedInsteadChildren()),
		solidTime:                cm.BlockDAG.Time,
	}

	return b
}

func blocksToBlockIDs(blocks []*blockdag.Block) models.BlockIDs {
	return models.NewBlockIDs(lo.Map(blocks, func(block *blockdag.Block) models.BlockID { return block.ID() })...)
}
