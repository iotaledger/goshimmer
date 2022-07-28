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
	*models.Block

	missing              bool
	solid                bool
	invalid              bool
	strongChildren       []*Block
	weakChildren         []*Block
	likedInsteadChildren []*Block

	*syncutils.StarvingMutex
}

func NewBlock(block *models.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block:                block,
		strongChildren:       make([]*Block, 0),
		weakChildren:         make([]*Block, 0),
		likedInsteadChildren: make([]*Block, 0),
		StarvingMutex:        syncutils.NewStarvingMutex(),
	}, opts)
}

// IsSolid returns true if the block is solid.
func (b *Block) IsSolid() bool {
	b.RLock()
	defer b.RUnlock()

	return b.isSolid()
}

// ParentIDs returns the parents of the block as a slice.
func (b *Block) ParentIDs() []models.BlockID {
	parents := b.ParentsByType(models.StrongParentType).Clone()
	parents.AddAll(b.ParentsByType(models.WeakParentType))
	parents.AddAll(b.ParentsByType(models.ShallowLikeParentType))

	return parents.Slice()
}

// Children returns the metadata of the children of the block.
func (b *Block) Children() (childrenMetadata []*Block) {
	b.RLock()
	defer b.RUnlock()

	return b.children()
}

func (b *Block) isSolid() bool {
	return b.solid
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
func (b *Block) children() (childrenMetadata []*Block) {
	seenBlockIDs := make(map[models.BlockID]types.Empty)
	for _, parentsByType := range [][]*Block{
		b.strongChildren,
		b.weakChildren,
		b.likedInsteadChildren,
	} {
		for _, childMetadata := range parentsByType {
			if _, exists := seenBlockIDs[childMetadata.ID()]; !exists {
				childrenMetadata = append(childrenMetadata, childMetadata)
				seenBlockIDs[childMetadata.ID()] = types.Void
			}
		}
	}

	return childrenMetadata
}

func (b *Block) publishMissingBlock(block *models.Block) (published bool) {
	if !b.missing {
		return
	}

	b.updateModel(block)
	b.missing = false

	return true
}

func (b *Block) updateModel(other *models.Block) {
	b.Block.Lock()
	defer b.Block.Unlock()

	b.M = other.M
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMissing(missing bool) options.Option[Block] {
	return func(block *Block) {
		block.missing = true
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Errors ///////////////////////////////////////////////////////////////////////////////////////////////////////

var GenesisMetadata = &Block{
	Block:                models.NewEmptyBlock(models.EmptyBlockID),
	strongChildren:       make([]*Block, 0),
	weakChildren:         make([]*Block, 0),
	likedInsteadChildren: make([]*Block, 0),
	solid:                true,
	StarvingMutex:        syncutils.NewStarvingMutex(),
}

// SolidEntrypointMetadata returns the metadata for a solid entrypoint.
func SolidEntrypointMetadata(blockID models.BlockID) *Block {
	return GenesisMetadata
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
