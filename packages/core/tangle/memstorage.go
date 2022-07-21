package tangle

import "github.com/iotaledger/goshimmer/packages/core/memstorage"

type MemStorage struct {
	metadataStorage *memstorage.EpochStorage[BlockID, *BlockMetadata]
}

func (s *MemStorage) GetBlockMetadata(blockID BlockID) *BlockMetadata {
	epochStorage, _ := s.metadataStorage.Get(blockID.EpochIndex, true)

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

func (s *MemStorage) PutBlockMetadata(blockMetadataNew *BlockMetadata) (added bool) {

	epochStorage, _ := s.metadataStorage.Get(blockMetadataNew.ID().EpochIndex, true)

	// lock
	epochStorage.Lock()
	defer epochStorage.Unlock()

	blockMetadataRetrieved, exists := epochStorage.Get(blockMetadataNew.ID())

	if !exists {
		epochStorage.Set(blockMetadataNew.ID(), blockMetadataNew)
		return true
	}

	if !blockMetadataRetrieved.missing {
		return
	}

	blockMetadataRetrieved.missing = false
	blockMetadataRetrieved.likedInsteadParents = blockMetadataNew.likedInsteadParents
	blockMetadataRetrieved.strongParents = blockMetadataNew.strongParents
	blockMetadataRetrieved.weakParents = blockMetadataNew.weakParents

	// TODO: trigger missing block received

	return true

	// Lock
	// GET METADATA OBJECT
	// IF EXISTS {
	//    IF MARKED AS MISSING {
	//       TRIGGER MISSING RECEIVED
	//       RETURN TRUE
	//   }
	// }
	// STORE + RETURN TRUE
}
