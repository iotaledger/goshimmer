package retainer

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Retainer struct {
	cachedMetadata *memstorage.EpochStorage[models.BlockID, *cachedMetadata]
	blockStorage   *database.PersistentEpochStorage[models.BlockID, BlockMetadata, *models.BlockID, *BlockMetadata]

	protocol     *protocol.Protocol
	evictionLock sync.RWMutex

	optsRealm kvstore.Realm
}

func NewRetainer(protocol *protocol.Protocol, dbManager *database.Manager, opts ...options.Option[Retainer]) (r *Retainer) {
	return options.Apply(&Retainer{
		cachedMetadata: memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		protocol:       protocol,
		optsRealm:      []byte("retainer"),
	}, opts, (*Retainer).setupEvents, func(r *Retainer) {
		r.blockStorage = database.NewPersistentEpochStorage[models.BlockID, BlockMetadata](dbManager, r.optsRealm)
	})
}

func (r *Retainer) Block(blockID models.BlockID) (block *models.Block, exists bool) {
	return r.protocol.Engine().Block(blockID)
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
		cm.setBlockDAGBlock(block)
	}))

	r.protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBookerBlock(block)

		cm.ConflictIDs = r.protocol.Engine().Tangle.BlockConflicts(block)
	}))

	r.protocol.Events.Engine.Tangle.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setVirtualVotingBlock(block)
	}))

	congestionControlClosure := event.NewClosure(func(block *scheduler.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setSchedulerBlock(block)
	})
	r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(congestionControlClosure)
	r.protocol.Events.CongestionControl.Scheduler.BlockDropped.Attach(congestionControlClosure)
	r.protocol.Events.CongestionControl.Scheduler.BlockSkipped.Attach(congestionControlClosure)

	r.protocol.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setAcceptanceBlock(block)
	}))

	r.protocol.Events.Engine.EvictionManager.EpochEvicted.Attach(event.NewClosure(r.storeAndEvictEpoch))
}

func (r *Retainer) createOrGetCachedMetadata(id models.BlockID) *cachedMetadata {
	r.evictionLock.RLock()
	defer r.evictionLock.RUnlock()

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
	r.evictionLock.Lock()
	defer r.evictionLock.Unlock()
	r.cachedMetadata.EvictEpoch(epochIndex)
}

func (r *Retainer) createStorableBlockMetadata(epochIndex epoch.Index) (metas []*BlockMetadata) {
	r.evictionLock.RLock()
	defer r.evictionLock.RUnlock()

	storage := r.cachedMetadata.Get(epochIndex)
	if storage == nil {
		return metas
	}

	metas = make([]*BlockMetadata, 0, storage.Size())
	storage.ForEach(func(blockID models.BlockID, cm *cachedMetadata) bool {
		blockMetadata := newBlockMetadata(cm)
		blockMetadata.M.ConflictIDs = r.protocol.Engine().Tangle.BlockConflicts(cm.Booker.Block)

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
	r.evictionLock.RLock()
	defer r.evictionLock.RUnlock()

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
