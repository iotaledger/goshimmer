package tangle

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with Tangle related metadata.
type Block struct {
	missing                   bool
	solid                     bool
	invalid                   bool
	markedOrphaned            bool
	orphanedParentsInPastCone models.BlockIDs
	strongChildren            []*Block
	weakChildren              []*Block
	likedInsteadChildren      []*Block

	*models.Block
}

// NewBlock creates a new Block with the given options.
func NewBlock(data *models.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		strongChildren:            make([]*Block, 0),
		weakChildren:              make([]*Block, 0),
		likedInsteadChildren:      make([]*Block, 0),
		orphanedParentsInPastCone: models.NewBlockIDs(),
		Block:                     data,
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

	return b.markedOrphaned || !b.orphanedParentsInPastCone.Empty()
}

// IsMarkedOrphaned returns true if the Block is marked as orphaned itself.
func (b *Block) IsMarkedOrphaned() (isOrphaned bool) {
	b.RLock()
	defer b.RUnlock()

	return b.markedOrphaned
}

// OrphanedParentsInPastCone returns the list of orphaned parents in the past cone of the Block.
func (b *Block) OrphanedParentsInPastCone() (orphanedParentsInPastCone models.BlockIDs) {
	b.RLock()
	defer b.RUnlock()

	return b.orphanedParentsInPastCone
}

// Children returns the children of the Block.
func (b *Block) Children() (children []*Block) {
	b.RLock()
	defer b.RUnlock()

	seenBlockIDs := make(map[models.BlockID]types.Empty)
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

// setSolid marks the Block as solid. It is private even though it locks because we want to prevent people from
// setting the solid flag manually.
func (b *Block) setSolid() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.solid; wasUpdated {
		b.solid = true
	}

	return
}

// setInvalid marks the Block as invalid. It is private even though it locks because we want to prevent people from
// setting the invalid flag manually.
func (b *Block) setInvalid() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if b.invalid {
		return false
	}

	b.invalid = true

	return true
}

// setMarkedOrphaned marks a block as orphaned.
func (b *Block) setMarkedOrphaned(orphaned bool) (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if b.markedOrphaned == orphaned {
		return false
	}

	b.markedOrphaned = orphaned

	return true
}

// addOrphanedParents adds the given BlockID to the list of orphaned parents in the past cone.
func (b *Block) addOrphanedParentInPastCone(id models.BlockID) (wasAdded bool, becameOrphaned bool) {
	b.Lock()
	defer b.Unlock()

	if b.orphanedParentsInPastCone.Contains(id) {
		return false, false
	}

	return true, len(b.orphanedParentsInPastCone.Add(id)) == 1 && !b.markedOrphaned
}

// removeOrphanedParentInPastCone removes the given BlockID from the list of orphaned parents in the past cone.
func (b *Block) removeOrphanedParentInPastCone(id models.BlockID) (wasRemoved bool, becameUnorphaned bool) {
	b.Lock()
	defer b.Unlock()

	if !b.orphanedParentsInPastCone.Contains(id) {
		return false, false
	}

	return true, len(b.orphanedParentsInPastCone.Remove(id)) == 0 && !b.markedOrphaned
}

// appendChild adds a child of the corresponding type to the Block.
func (b *Block) appendChild(child *Block, childType models.ParentsType) {
	b.Lock()
	defer b.Unlock()

	switch childType {
	case models.StrongParentType:
		b.strongChildren = append(b.strongChildren, child)
	case models.WeakParentType:
		b.weakChildren = append(b.weakChildren, child)
	case models.ShallowLikeParentType:
		b.likedInsteadChildren = append(b.likedInsteadChildren, child)
	}
}

// update publishes the given Block data to the underlying Block and marks it as no longer missing.
func (b *Block) update(data *models.Block) (wasPublished bool) {
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

func WithModelOptions(opts ...options.Option[models.Block]) options.Option[Block] {
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

// WithMarkedOrphaned is a constructor Option for Blocks that initializes the given block with a specific markedOrphaned
// flag.
func WithMarkedOrphaned(markedOrphaned bool) options.Option[Block] {
	return func(block *Block) {
		block.markedOrphaned = markedOrphaned
	}
}

// WithOrphanedParentsInPastCone is a constructor Option for Blocks that initializes the given Block with a list of
// orphaned parents in its past cone.
func WithOrphanedParentsInPastCone(orphanedParentsInPastCone models.BlockIDs) options.Option[Block] {
	return func(block *Block) {
		block.orphanedParentsInPastCone = orphanedParentsInPastCone
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
