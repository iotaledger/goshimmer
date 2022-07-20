package tangle

import "github.com/iotaledger/goshimmer/packages/core/memstorage"

type MemStorage struct {
	metadataStorage *memstorage.EpochStorage[BlockID, *BlockMetadata]
}

func (s *MemStorage) GetBlockMetadata(blockID BlockID) *BlockMetadata {
	epochStorage, _ := s.metadataStorage.Get(blockID.Epoch(), true)

	// lock
	blockMetadata, exists := epochStorage.Get(blockID)
	if exists {
		return blockMetadata
	}

	newBlockMetadata := &BlockMetadata{
		id:                   blockID,
		missing:              true,
		strongParents:        nil,
		weakParents:          nil,
		likedInsteadParents:  nil,
		strongChildren:       make([]*BlockMetadata, 0),
		weakChildren:         make([]*BlockMetadata, 0),
		likedInsteadChildren: make([]*BlockMetadata, 0),
	}

	epochStorage.Set(blockID, newBlockMetadata)

	return newBlockMetadata
}

func (s *MemStorage) PutBlockMetadata(blockMetadata *BlockMetadata) (added bool) {

	epochStorage, _ := s.metadataStorage.Get(blockMetadata.ID().Epoch(), true)

	// lock
	blockMetadata, exists := epochStorage.Get(blockMetadata.ID())
	if !exists {
		epochStorage.Set(blockMetadata.ID(), blockMetadata)
		added = true
	}

	// Lock
	// GET METADATA OBJECT
	// IF EXISTS {
	//    IF MARKED AS MISSING {
	//       TRIGGER MISSING RECEIVED
	//       RETURN TRUE
	//   }
	// }
	// STORE + RETURN TRUE

	return epochStorage.StoreIfAbsent(blockMetadata.ID(), blockMetadata)
}
