package retainer

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
)

type Retainer struct {
	cachedMetadata *memstorage.EpochStorage[models.BlockID, *cachedMetadata]
	blockStorage   *database.PersistentEpochStorage[models.BlockID, BlockMetadata, *BlockMetadata]

	protocol        *protocol.Protocol
	evictionManager *eviction.LockableManager[models.BlockID]

	optsRealm kvstore.Realm
}

func NewRetainer(protocol *protocol.Protocol, dbManager *database.Manager, opts ...options.Option[Retainer]) (r *Retainer) {
	return options.Apply(&Retainer{
		cachedMetadata:  memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		protocol:        protocol,
		evictionManager: protocol.EvictionManager.Lockable(),
		optsRealm:       []byte("retainer"),
	}, opts, (*Retainer).setupEvents, func(r *Retainer) {
		r.blockStorage = database.New[models.BlockID, BlockMetadata, *BlockMetadata](dbManager, r.optsRealm)
	})
}

func (r *Retainer) BlockMetadata(blockID models.BlockID) (metadata *BlockMetadata, exists bool) {
	if storageExists, blockMetadata, blockExists := r.blockMetadataFromCache(blockID); storageExists {
		return blockMetadata, blockExists
	}

	return r.blockStorage.Get(blockID)
}

func (r *Retainer) setupEvents() {
	r.protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlock(block, cm.BlockDAG)
	}))

	r.protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlock(block, cm.Booker)
	}))

	r.protocol.Events.Engine.Tangle.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlock(block, cm.VirtualVoting)
	}))

	congestionControlClosure := event.NewClosure(func(block *scheduler.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlock(block, cm.Scheduler)
	})
	r.protocol.Events.Engine.CongestionControl.Scheduler.BlockScheduled.Attach(congestionControlClosure)
	r.protocol.Events.Engine.CongestionControl.Scheduler.BlockDropped.Attach(congestionControlClosure)
	r.protocol.Events.Engine.CongestionControl.Scheduler.BlockSkipped.Attach(congestionControlClosure)

	r.protocol.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlock(block, cm.Acceptance)
	}))
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

	metas = make([]*BlockMetadata, 0, storage.Size())
	storage.ForEach(func(blockID models.BlockID, cm *cachedMetadata) bool {
		metas = append(metas, newBlockMetadata(cm))
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
