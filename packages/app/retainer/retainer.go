package retainer

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
)

type Retainer struct {
	cachedMetadata *memstorage.EpochStorage[models.BlockID, *cachedMetadata]
	blockStorage   *database.PersistentEpochStorage[models.BlockID, BlockMetadata, *BlockMetadata]

	protocol        *protocol.Protocol
	evictionManager *eviction.LockableManager[models.BlockID]
}

func NewRetainer(protocol *protocol.Protocol) (r *Retainer) {
	r = &Retainer{
		cachedMetadata: memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		// TODO: blockStorage: database.NewPersistentEpochStorage[models.BlockID, BlockMetadata, *BlockMetadata](),
		protocol:        protocol,
		evictionManager: protocol.EvictionManager.Lockable(),
	}

	r.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(r.storeAndEvictEpoch))

	// TODO: attach to events and store in memstorage
	//  booked
	//  tracked
	//  scheduled
	//  accepted
	r.protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		r.evictionManager.RLock()
		defer r.evictionManager.RUnlock()

		storage := r.cachedMetadata.Get(block.ID().Index(), true)
		cm, _ := storage.RetrieveOrCreate(block.ID(), newCachedMetadata)
		cm.BlockDAG = newBlockWithTime(block)
	}))

	return
}

func (r *Retainer) storeAndEvictEpoch(epochIndex epoch.Index) {
	// First we read the data from storage.
	metas := r.createStorableBlockMetadata(epochIndex)

	// Now we store it to disk (slow).
	r.storeBlockMetadata(metas)

	// Once everything is stored to disk, we evict it from cache.
	// Therefore, we make sure that we can always first try to read BlockMetadata from cache and if it's not in cache
	// anymore it is already written to disk.
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()
	r.cachedMetadata.EvictEpoch(epochIndex)
}

func (r *Retainer) createStorableBlockMetadata(epochIndex epoch.Index) (metas []*BlockMetadata) {
	r.evictionManager.RLock()
	defer r.evictionManager.RUnlock()

	storage := r.cachedMetadata.Get(epochIndex)

	metas = make([]*BlockMetadata, 0, storage.Size())
	storage.ForEach(func(blockID models.BlockID, cm *cachedMetadata) bool {
		metas = append(metas, newBlockMetadata(cm))
		return true
	})

	return metas
}

func (r *Retainer) storeBlockMetadata(metas []*BlockMetadata) {
	for _, meta := range metas {
		r.blockStorage.Set(meta.BlockID, meta)
	}
}

func (r *Retainer) BlockMetadata(blockID models.BlockID) (metadata *BlockMetadata, exists bool) {
	if storageExists, blockMetadata, blockExists := r.blockMetadataFromCache(blockID); storageExists {
		return blockMetadata, blockExists
	}

	return r.blockStorage.Get(blockID)
}

func (r *Retainer) blockMetadataFromCache(blockID models.BlockID) (storageExists bool, metadata *BlockMetadata, exists bool) {
	r.evictionManager.RLock()
	defer r.evictionManager.RUnlock()

	storage := r.cachedMetadata.Get(blockID.Index())
	if storage == nil {
		return false, nil, false
	}

	cm, exists := storage.Get(blockID)
	return true, newBlockMetadata(cm), exists
}
