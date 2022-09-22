package retainer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Retainer struct {
	cachedMetadata *memstorage.EpochStorage[models.BlockID, *cachedMetadata]
	blockStorage   *database.PersistentEpochStorage[models.BlockID, BlockMetadata, *models.BlockID, *BlockMetadata]

	engine          *engine.Engine
	evictionManager *eviction.LockableManager[models.BlockID]

	optsRealm kvstore.Realm
}

func NewRetainer(engine *engine.Engine, dbManager *database.Manager, opts ...options.Option[Retainer]) (r *Retainer) {
	return options.Apply(&Retainer{
		cachedMetadata:  memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		engine:          engine,
		evictionManager: engine.Tangle.EvictionManager.Lockable(),
		optsRealm:       []byte("retainer"),
	}, opts, (*Retainer).setupEvents, func(r *Retainer) {
		r.blockStorage = database.NewPersistentEpochStorage[models.BlockID, BlockMetadata, *models.BlockID, *BlockMetadata](dbManager, r.optsRealm)
	})
}

func (r *Retainer) BlockMetadata(blockID models.BlockID) (metadata *BlockMetadata, exists bool) {
	if storageExists, blockMetadata, blockExists := r.blockMetadataFromCache(blockID); storageExists {
		return blockMetadata, blockExists
	}

	return r.blockStorage.Get(blockID)
}

func (r *Retainer) setupEvents() {
	r.engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlockDAGBlock(block)
	}))

	r.engine.Events.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBookerBlock(block)

		cm.ConflictIDs = r.engine.Tangle.BlockConflicts(block)
	}))

	r.engine.Events.Tangle.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setVirtualVotingBlock(block)
	}))

	congestionControlClosure := event.NewClosure(func(block *scheduler.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setSchedulerBlock(block)
	})
	r.engine.Events.CongestionControl.Scheduler.BlockScheduled.Attach(congestionControlClosure)
	r.engine.Events.CongestionControl.Scheduler.BlockDropped.Attach(congestionControlClosure)
	r.engine.Events.CongestionControl.Scheduler.BlockSkipped.Attach(congestionControlClosure)

	r.engine.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setAcceptanceBlock(block)
	}))

	r.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(r.storeAndEvictEpoch))
}

func (r *Retainer) createOrGetCachedMetadata(id models.BlockID) *cachedMetadata {
	r.evictionManager.RLock()
	defer r.evictionManager.RUnlock()

	storage := r.cachedMetadata.Get(id.Index(), true)
	cm, _ := storage.RetrieveOrCreate(id, newCachedMetadata)
	return cm
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
	if storage == nil {
		return metas
	}

	metas = make([]*BlockMetadata, 0, storage.Size())
	storage.ForEach(func(blockID models.BlockID, cm *cachedMetadata) bool {
		blockMetadata := newBlockMetadata(cm)
		blockMetadata.M.ConflictIDs = r.engine.Tangle.BlockConflicts(cm.Booker.Block)

		metas = append(metas, blockMetadata)
		return true
	})

	return metas
}

func (r *Retainer) storeBlockMetadata(metas []*BlockMetadata) {
	for _, meta := range metas {
		r.blockStorage.Set(meta.ID(), meta)
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

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRealm(realm kvstore.Realm) options.Option[Retainer] {
	return func(r *Retainer) {
		r.optsRealm = realm
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
