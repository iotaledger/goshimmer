package tangle

import (
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with the tangle related metadata.
type Block struct {
	missing              bool
	solid                bool
	invalid              bool
	root                 bool
	strongChildren       []*Block
	weakChildren         []*Block
	likedInsteadChildren []*Block

	*models.Block
	*syncutils.StarvingMutex
}

func NewBlock(block *models.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		strongChildren:       make([]*Block, 0),
		weakChildren:         make([]*Block, 0),
		likedInsteadChildren: make([]*Block, 0),
		Block:                block,
		StarvingMutex:        syncutils.NewStarvingMutex(),
	}, opts)
}

func (b *Block) IsRoot() (isRoot bool) {
	b.RLock()
	defer b.RUnlock()

	return b.root
}

// IsMissing returns a flag that indicates if the underlying block data hasn't been stored, yet.
func (b *Block) IsMissing() (isMissing bool) {
	b.RLock()
	defer b.RUnlock()

	return b.missing
}

// IsSolid returns true if the block is solid.
func (b *Block) IsSolid() (isSolid bool) {
	b.RLock()
	defer b.RUnlock()

	return b.solid
}

// IsInvalid returns true if the block is solid.
func (b *Block) IsInvalid() (isInvalid bool) {
	b.RLock()
	defer b.RUnlock()

	return b.invalid
}

// Children returns the metadata of the children of the block.
func (b *Block) Children() (children []*Block) {
	b.RLock()
	defer b.RUnlock()

	return b.children()
}

func (b *Block) setSolid(solid bool) (updated bool) {
	if b.solid == solid {
		return false
	}

	b.solid = solid

	return true
}

func (b *Block) setInvalid() (updated bool) {
	b.Lock()
	defer b.Unlock()

	if b.invalid {
		return
	}

	b.invalid = true

	return true
}

// Children returns the metadata of the children of the block.
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

func (b *Block) appendChild(block *Block, parentType models.ParentsType) {
	b.Lock()
	defer b.Unlock()

	switch parentType {
	case models.StrongParentType:
		b.strongChildren = append(b.strongChildren, block)
	case models.WeakParentType:
		b.weakChildren = append(b.weakChildren, block)
	case models.ShallowLikeParentType:
		b.likedInsteadChildren = append(b.likedInsteadChildren, block)
	}
}

func (b *Block) publishBlockData(data *models.Block) (wasPublished bool) {
	data.Lock()
	defer data.Unlock()

	if !b.missing {
		return
	}

	b.Block.Lock()
	defer b.Block.Unlock()

	b.M = data.M
	b.missing = false

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMissing(missing bool) options.Option[Block] {
	return func(block *Block) {
		block.missing = missing
	}
}

func WithSolid(solid bool) options.Option[Block] {
	return func(block *Block) {
		block.solid = solid
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
