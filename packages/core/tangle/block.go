package tangle

import (
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with Tangle related metadata.
type Block struct {
	missing              bool
	solid                bool
	invalid              bool
	strongChildren       []*Block
	weakChildren         []*Block
	likedInsteadChildren []*Block

	*models.Block
	*syncutils.StarvingMutex
}

// NewBlock creates a new Block with the given options.
func NewBlock(data *models.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		strongChildren:       make([]*Block, 0),
		weakChildren:         make([]*Block, 0),
		likedInsteadChildren: make([]*Block, 0),
		Block:                data,
		StarvingMutex:        syncutils.NewStarvingMutex(),
	}, opts)
}

// IsMissing returns a flag that indicates if the underlying Block data hasn't been stored, yet.
func (b *Block) IsMissing() (isMissing bool) {
	b.RLock()
	defer b.RUnlock()

	return b.missing
}

// IsSolid returns true if the Block is solid (the entire causal history is known)
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

// Children returns the children of the Block.
func (b *Block) Children() (children []*Block) {
	b.RLock()
	defer b.RUnlock()

	return b.children()
}

// setInvalid marks the Block as invalid. This is private even though it locks because we want to prevent people from
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

// children returns the children of the Block (without locking).
func (b *Block) children() (children []*Block) {
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

	b.Block.Lock()
	defer b.Block.Unlock()

	b.M = data.M

	return true
}

// solidPtr returns a pointer to the solid flag.
func (b *Block) solidPtr() (solidPtr *bool) {
	return &b.solid
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
