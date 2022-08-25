package retainer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

var retainerRealm = kvstore.Realm("retainer")

type Retainer struct {
	cachedMetadata *memstorage.EpochStorage[models.BlockID, *cachedMetadata]

	dbManager       *database.Manager
	evictionManager *eviction.LockableManager[models.BlockID]
}

func NewRetainer(dbManager *database.Manager, evictionManager *eviction.Manager[models.BlockID]) (r *Retainer) {
	r = &Retainer{
		dbManager:       dbManager,
		cachedMetadata:  memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		evictionManager: evictionManager.Lockable(),
	}

	r.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(r.storeAndEvictEpoch))

	// TODO: attach to events and store in memstorage
	//  solid
	//  booked
	//  tracked
	//  scheduled
	//  accepted

	return
}

func (r *Retainer) storeAndEvictEpoch(epochIndex epoch.Index) {
	metadata := r.createStorableMetadata(epochIndex)

	// TODO: store cachedMetadata to disk
	//  should we use object storage or just plain KV store?

	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()
	r.cachedMetadata.EvictEpoch(epochIndex)
}

func (r *Retainer) createStorableMetadata(epochIndex epoch.Index) (metas []*BlockMetadata) {
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

func (r *Retainer) BlockMetadata(blockID models.BlockID) (metadata *BlockMetadata, exists bool) {
	if storageExists, blockMetadata, blockExists := r.blockMetadataFromCache(blockID); storageExists {
		return blockMetadata, blockExists
	}

	// TODO: read from KV store
	kv := r.dbManager.Get(blockID.Index(), retainerRealm)
	blockMeta, err := kv.Get(blockID.Bytes())
	if err != nil {
		return nil, false
	}

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
