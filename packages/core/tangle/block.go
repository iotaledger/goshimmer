package tangle

import (
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region BlockMetadata ///////////////////////////////////////////////////////////////////////////////////////////////////////

// SolidifiedBlock represents a Block that was annotated with the Solidification based metadata.
type SolidifiedBlock struct {
	*models.Block

	id                   models.BlockID
	missing              bool
	solid                bool
	invalid              bool
	strongChildren       []*SolidifiedBlock
	weakChildren         []*SolidifiedBlock
	likedInsteadChildren []*SolidifiedBlock

	*syncutils.StarvingMutex
}

func fullMetadataFromBlock(block *models.Block) func() *SolidifiedBlock {
	return func() *SolidifiedBlock {
		return &SolidifiedBlock{
			Block: block,

			id:                   block.ID(),
			strongChildren:       make([]*SolidifiedBlock, 0),
			weakChildren:         make([]*SolidifiedBlock, 0),
			likedInsteadChildren: make([]*SolidifiedBlock, 0),
			StarvingMutex:        syncutils.NewStarvingMutex(),
		}
	}
}

func (b *SolidifiedBlock) ID() models.BlockID {
	return b.id
}

// Initialized returns true if the block metadata is initialized.
func (b *SolidifiedBlock) Initialized() bool {
	return b.solid || b.invalid
}

// IsSolid returns true if the block is solid.
func (b *SolidifiedBlock) IsSolid() bool {
	b.RLock()
	defer b.RUnlock()

	return b.isSolid()
}

func (b *SolidifiedBlock) isSolid() bool {
	return b.solid
}

func (b *SolidifiedBlock) setSolid(solid bool) (updated bool) {
	if b.solid == solid {
		return false
	}

	b.solid = solid

	return true
}

func (b *SolidifiedBlock) setInvalid() (updated bool) {
	if b.invalid {
		return false
	}

	b.invalid = true

	return true
}

// ParentIDs returns the parents of the block as a slice.
func (b *SolidifiedBlock) ParentIDs() []models.BlockID {
	parents := b.ParentsByType(models.StrongParentType).Clone()
	parents.AddAll(b.ParentsByType(models.WeakParentType))
	parents.AddAll(b.ParentsByType(models.ShallowLikeParentType))

	return parents.Slice()
}

// Children returns the metadata of the children of the block.
func (b *SolidifiedBlock) Children() (childrenMetadata []*SolidifiedBlock) {
	b.RLock()
	defer b.RUnlock()

	return b.children()
}

// Children returns the metadata of the children of the block.
func (b *SolidifiedBlock) children() (childrenMetadata []*SolidifiedBlock) {
	seenBlockIDs := make(map[models.BlockID]types.Empty)
	for _, parentsByType := range [][]*SolidifiedBlock{
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

var GenesisMetadata = &SolidifiedBlock{
	id:                   models.EmptyBlockID,
	strongChildren:       make([]*SolidifiedBlock, 0),
	weakChildren:         make([]*SolidifiedBlock, 0),
	likedInsteadChildren: make([]*SolidifiedBlock, 0),
	solid:                true,
	StarvingMutex:        syncutils.NewStarvingMutex(),
}

// SolidEntrypointMetadata returns the metadata for a solid entrypoint.
func SolidEntrypointMetadata(blockID models.BlockID) *SolidifiedBlock {
	return GenesisMetadata
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
