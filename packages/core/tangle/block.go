package tangle

import (
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/core/models"
)

// region BlockMetadata ///////////////////////////////////////////////////////////////////////////////////////////////////////

// BlockMetadata defines the metadata for a block.
type BlockMetadata struct {
	*models.Block

	id                   models.BlockID
	missing              bool
	solid                bool
	invalid              bool
	strongChildren       []*BlockMetadata
	weakChildren         []*BlockMetadata
	likedInsteadChildren []*BlockMetadata

	*syncutils.StarvingMutex
}

func fullMetadataFromBlock(block *models.Block) func() *BlockMetadata {
	return func() *BlockMetadata {
		return &BlockMetadata{
			Block: block,

			id:                   block.ID(),
			strongChildren:       make([]*BlockMetadata, 0),
			weakChildren:         make([]*BlockMetadata, 0),
			likedInsteadChildren: make([]*BlockMetadata, 0),
			StarvingMutex:        syncutils.NewStarvingMutex(),
		}
	}
}

func (b *BlockMetadata) ID() models.BlockID {
	return b.id
}

// Initialized returns true if the block metadata is initialized.
func (b *BlockMetadata) Initialized() bool {
	return b.solid || b.invalid
}

// IsSolid returns true if the block is solid.
func (b *BlockMetadata) IsSolid() bool {
	b.RLock()
	defer b.RUnlock()

	return b.isSolid()
}

func (b *BlockMetadata) isSolid() bool {
	return b.solid
}

func (b *BlockMetadata) setSolid(solid bool) (updated bool) {
	if b.solid == solid {
		return false
	}

	b.solid = solid

	return true
}

func (b *BlockMetadata) setInvalid() (updated bool) {
	if b.invalid {
		return false
	}

	b.invalid = true

	return true
}

// ParentIDs returns the parents of the block as a slice.
func (b *BlockMetadata) ParentIDs() []models.BlockID {
	parents := b.ParentsByType(models.StrongParentType).Clone()
	parents.AddAll(b.ParentsByType(models.WeakParentType))
	parents.AddAll(b.ParentsByType(models.ShallowLikeParentType))

	return parents.Slice()
}

// Children returns the metadata of the children of the block.
func (b *BlockMetadata) Children() (childrenMetadata []*BlockMetadata) {
	b.RLock()
	defer b.RUnlock()

	return b.children()
}

// Children returns the metadata of the children of the block.
func (b *BlockMetadata) children() (childrenMetadata []*BlockMetadata) {
	seenBlockIDs := make(map[models.BlockID]types.Empty)
	for _, parentsByType := range [][]*BlockMetadata{
		b.strongChildren,
		b.weakChildren,
		b.likedInsteadChildren,
	} {
		for _, childMetadata := range parentsByType {
			if _, exists := seenBlockIDs[childMetadata.id]; !exists {
				childrenMetadata = append(childrenMetadata, childMetadata)
				seenBlockIDs[childMetadata.id] = types.Void
			}
		}
	}

	return childrenMetadata
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Errors ///////////////////////////////////////////////////////////////////////////////////////////////////////

var GenesisMetadata = &BlockMetadata{
	id:                   models.EmptyBlockID,
	strongChildren:       make([]*BlockMetadata, 0),
	weakChildren:         make([]*BlockMetadata, 0),
	likedInsteadChildren: make([]*BlockMetadata, 0),
	solid:                true,
	StarvingMutex:        syncutils.NewStarvingMutex(),
}

// SolidEntrypointMetadata returns the metadata for a solid entrypoint.
func SolidEntrypointMetadata(blockID models.BlockID) *BlockMetadata {
	return GenesisMetadata
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
