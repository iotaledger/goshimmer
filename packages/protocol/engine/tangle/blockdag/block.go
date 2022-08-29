package blockdag

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/types"

	models2 "github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with Tangle related metadata.
type Block struct {
	missing                  bool
	solid                    bool
	invalid                  bool
	orphaned                 bool
	orphanedBlocksInPastCone models2.BlockIDs
	strongChildren           []*Block
	weakChildren             []*Block
	likedInsteadChildren     []*Block

	*models2.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(data *models2.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		orphanedBlocksInPastCone: models2.NewBlockIDs(),
		strongChildren:           make([]*Block, 0),
		weakChildren:             make([]*Block, 0),
		likedInsteadChildren:     make([]*Block, 0),
		Block:                    data,
	}, opts)
}

// IsMissing returns a flag that indicates if the underlying Block data hasn't been stored, yet.
func (b *Block) IsMissing() (isMissing bool) {
	b.RLock()
	defer b.RUnlock()

	return b.missing
}

// IsSolid returns true if the Block is solid (the entire causal history is known).
func (b *Block) IsSolid() (isSolid bool) {
	b.RLock()
	defer b.RUnlock()

	return b.solid
}

// IsInvalid returns true if the Block was marked as invalid.
func (b *Block) IsInvalid() (isInvalid bool) {
	b.RLock()
	defer b.RUnlock()

	return b.invalid
}

// IsOrphaned returns true if the Block is orphaned (either due to being marked as orphaned itself or because it has
// orphaned Blocks in its past cone).
func (b *Block) IsOrphaned() (isOrphaned bool) {
	b.RLock()
	defer b.RUnlock()

	return b.orphaned || !b.orphanedBlocksInPastCone.Empty()
}

// OrphanedBlocksInPastCone returns the list of orphaned Blocks in the Blocks past cone.
func (b *Block) OrphanedBlocksInPastCone() (orphanedBlocks models2.BlockIDs) {
	b.RLock()
	defer b.RUnlock()

	return b.orphanedBlocksInPastCone
}

// Children returns the children of the Block.
func (b *Block) Children() (children []*Block) {
	b.RLock()
	defer b.RUnlock()

	seenBlockIDs := make(map[models2.BlockID]types.Empty)
	for _, parentsByType := range [][]*Block{
		b.strongChildren,
		b.weakChildren,
		b.likedInsteadChildren,
	} {
		for _, childMetadata := range parentsByType {
			if _, exists := seenBlockIDs[childMetadata.ID()]; !exists {
				children = append(children, childMetadata)
				seenBlockIDs[childMetadata.ID()] = types.Void
			}
		}
	}

	return children
}

func (b *Block) StrongChildren() []*Block {
	b.RLock()
	defer b.RUnlock()
	return b.strongChildren
}

func (b *Block) WeakChildren() []*Block {
	b.RLock()
	defer b.RUnlock()
	return b.weakChildren
}

func (b *Block) LikedInsteadChildren() []*Block {
	b.RLock()
	defer b.RUnlock()
	return b.likedInsteadChildren
}

// setSolid marks the Block as solid.
func (b *Block) setSolid() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.solid; wasUpdated {
		b.solid = true
	}

	return
}

// setInvalid marks the Block as invalid.
func (b *Block) setInvalid() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if b.invalid {
		return false
	}

	b.invalid = true

	return true
}

func (b *Block) isOrphaned() (isOrphaned bool) {
	b.RLock()
	defer b.RUnlock()

	return b.orphaned
}

// setOrphaned sets the orphaned flag of the Block.
func (b *Block) setOrphaned(orphaned bool) (wasFlagUpdated bool, wasOrphanedUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if b.orphaned == orphaned {
		return false, false
	}
	b.orphaned = orphaned

	return true, b.orphanedBlocksInPastCone.Empty()
}

// addOrphanedBlocksInPastCone adds the given BlockIDs to the list of orphaned Blocks in the past cone.
func (b *Block) addOrphanedBlocksInPastCone(ids models2.BlockIDs) (wasAdded bool, becameOrphaned bool) {
	b.Lock()
	defer b.Unlock()

	initialCount := len(b.orphanedBlocksInPastCone)
	b.orphanedBlocksInPastCone.AddAll(ids)
	newCount := len(b.orphanedBlocksInPastCone)

	return newCount > initialCount, initialCount == 0 && newCount != 0 && !b.orphaned
}

// removeOrphanedBlocksInPastCone removes the given BlockIDs from the list of orphaned Blocks in the past cone.
func (b *Block) removeOrphanedBlocksInPastCone(ids models2.BlockIDs) (wasRemoved bool, becameUnorphaned bool) {
	b.Lock()
	defer b.Unlock()

	initialCount := len(b.orphanedBlocksInPastCone)
	b.orphanedBlocksInPastCone.RemoveAll(ids)
	newCount := len(b.orphanedBlocksInPastCone)

	return newCount < initialCount, initialCount != 0 && newCount == 0 && !b.orphaned
}

// appendChild adds a child of the corresponding type to the Block.
func (b *Block) appendChild(child *Block, childType models2.ParentsType) {
	b.Lock()
	defer b.Unlock()

	switch childType {
	case models2.StrongParentType:
		b.strongChildren = append(b.strongChildren, child)
	case models2.WeakParentType:
		b.weakChildren = append(b.weakChildren, child)
	case models2.ShallowLikeParentType:
		b.likedInsteadChildren = append(b.likedInsteadChildren, child)
	}
}

// update publishes the given Block data to the underlying Block and marks it as no longer missing.
func (b *Block) update(data *models2.Block) (wasPublished bool) {
	b.Lock()
	defer b.Unlock()

	if !b.missing {
		return
	}

	b.missing = false
	b.M = data.M

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithModelOptions(opts ...options.Option[models2.Block]) options.Option[Block] {
	return func(block *Block) {
		options.Apply(block.Block, opts)
	}
}

// WithMissing is a constructor Option for Blocks that initializes the given block with a specific missing flag.
func WithMissing(missing bool) options.Option[Block] {
	return func(block *Block) {
		block.missing = missing
	}
}

// WithSolid is a constructor Option for Blocks that initializes the given block with a specific solid flag.
func WithSolid(solid bool) options.Option[Block] {
	return func(block *Block) {
		block.solid = solid
	}
}

// WithOrphaned is a constructor Option for Blocks that initializes the given block with a specific orphaned flag.
func WithOrphaned(markedOrphaned bool) options.Option[Block] {
	return func(block *Block) {
		block.orphaned = markedOrphaned
	}
}

// WithOrphanedBlocksInPastCone is a constructor Option for Blocks that initializes the given Block with a list of
// orphaned Blocks in its past cone.
func WithOrphanedBlocksInPastCone(orphanedBlocks models2.BlockIDs) options.Option[Block] {
	return func(block *Block) {
		block.orphanedBlocksInPastCone = orphanedBlocks
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
