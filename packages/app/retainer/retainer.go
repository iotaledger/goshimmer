package retainer

import (
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Retainer struct {
	workerPool     *workerpool.UnboundedWorkerPool
	cachedMetadata *memstorage.EpochStorage[models.BlockID, *cachedMetadata]
	blockStorage   *database.PersistentEpochStorage[models.BlockID, BlockMetadata, *models.BlockID, *BlockMetadata]

	dbManager    *database.Manager
	protocol     *protocol.Protocol
	evictionLock *syncutils.DAGMutex[epoch.Index]

	optsRealm kvstore.Realm
}

func NewRetainer(protocol *protocol.Protocol, dbManager *database.Manager, opts ...options.Option[Retainer]) (r *Retainer) {
	return options.Apply(&Retainer{
		workerPool:     workerpool.NewUnboundedWorkerPool(2),
		cachedMetadata: memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		protocol:       protocol,
		dbManager:      dbManager,
		optsRealm:      []byte("retainer"),
	}, opts, (*Retainer).setupEvents, func(r *Retainer) {
		r.blockStorage = database.NewPersistentEpochStorage[models.BlockID, BlockMetadata](dbManager, r.optsRealm)
		r.evictionLock = syncutils.NewDAGMutex[epoch.Index]()
	})
}

func (r *Retainer) Shutdown() {
	r.workerPool.Shutdown()
}

func (r *Retainer) Block(blockID models.BlockID) (block *models.Block, exists bool) {
	if metadata, metadataExists := r.BlockMetadata(blockID); metadataExists {
		return metadata.M.Block, metadata.M.Block != nil
	}
	return nil, false
}

func (r *Retainer) BlockMetadata(blockID models.BlockID) (metadata *BlockMetadata, exists bool) {
	if storageExists, blockMetadata, blockExists := r.blockMetadataFromCache(blockID); storageExists {
		return blockMetadata, blockExists
	}

	metadata = new(BlockMetadata)
	*metadata, exists = r.blockStorage.Get(blockID)
	if exists {
		metadata.SetID(metadata.M.ID)

		if metadata.M.Accepted && !metadata.M.Confirmed && blockID.Index() <= r.protocol.Engine().LastConfirmedEpoch() {
			metadata.M.ConfirmedByEpoch = true
			metadata.M.ConfirmedByEpochTime = blockID.Index().EndTime()
		}
	}

	return metadata, exists
}

func (r *Retainer) LoadAll(index epoch.Index) (ids *set.AdvancedSet[*BlockMetadata]) {
	r.evictionLock.RLock(index)
	defer r.evictionLock.RUnlock(index)

	ids = set.NewAdvancedSet[*BlockMetadata]()
	r.Stream(index, func(id models.BlockID, metadata *BlockMetadata) {
		ids.Add(metadata)
	})
	return
}

func (r *Retainer) Stream(index epoch.Index, callback func(id models.BlockID, metadata *BlockMetadata)) {
	r.evictionLock.RLock(index)
	defer r.evictionLock.RUnlock(index)

	if epochStorage := r.cachedMetadata.Get(index, false); epochStorage != nil {
		epochStorage.ForEach(func(id models.BlockID, cachedMetadata *cachedMetadata) bool {
			callback(id, newBlockMetadata(cachedMetadata))
			return true
		})
		return
	}

	_ = r.blockStorage.Iterate(index, func(id models.BlockID, metadata BlockMetadata) bool {
		callback(id, &metadata)
		return true
	})
}

// DatabaseSize returns the size of the underlying databases.
func (r *Retainer) DatabaseSize() int64 {
	return r.dbManager.TotalStorageSize()
}

// WorkerPool returns the worker pool of the retainer.
func (r *Retainer) WorkerPool() *workerpool.UnboundedWorkerPool {
	return r.workerPool
}

// PruneUntilEpoch prunes storage epochs less than and equal to the given index.
func (r *Retainer) PruneUntilEpoch(epochIndex epoch.Index) {
	r.dbManager.PruneUntilEpoch(epochIndex)
}

func (r *Retainer) setupEvents() {
	r.workerPool.Start()

	r.protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.AttachWithWorkerPool(event.NewClosure(func(block *blockdag.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBlockDAGBlock(block)
		}
	}), r.workerPool)

	// TODO: missing blocks make the node fail due to empty strong parents
	// r.protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.AttachWithWorkerPool(event.NewClosure(func(block *blockdag.Block) {
	//	cm := r.createOrGetCachedMetadata(block.ID())
	//	cm.setBlockDAGBlock(block)
	// }))

	r.protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.AttachWithWorkerPool(event.NewClosure(func(block *blockdag.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBlockDAGBlock(block)
		}
	}), r.workerPool)

	r.protocol.Events.Engine.Tangle.Booker.BlockBooked.AttachWithWorkerPool(event.NewClosure(func(block *booker.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBookerBlock(block)
			cm.Lock()
			cm.ConflictIDs = r.protocol.Engine().Tangle.BlockConflicts(block)
			cm.Unlock()
		}
	}), r.workerPool)

	r.protocol.Events.Engine.Tangle.VirtualVoting.BlockTracked.AttachWithWorkerPool(event.NewClosure(func(block *virtualvoting.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setVirtualVotingBlock(block)
		}
	}), r.workerPool)

	congestionControlClosure := event.NewClosure(func(block *scheduler.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setSchedulerBlock(block)
		}
	})
	r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.AttachWithWorkerPool(congestionControlClosure, r.workerPool)
	r.protocol.Events.CongestionControl.Scheduler.BlockDropped.AttachWithWorkerPool(congestionControlClosure, r.workerPool)
	r.protocol.Events.CongestionControl.Scheduler.BlockSkipped.AttachWithWorkerPool(congestionControlClosure, r.workerPool)

	r.protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.AttachWithWorkerPool(event.NewClosure(func(block *blockgadget.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setAcceptanceBlock(block)
	}), r.workerPool)

	r.protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.AttachWithWorkerPool(event.NewClosure(func(block *blockgadget.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setConfirmationBlock(block)
		}
	}), r.workerPool)

	r.protocol.Events.Engine.EvictionState.EpochEvicted.Hook(event.NewClosure(r.storeAndEvictEpoch))
}

func (r *Retainer) createOrGetCachedMetadata(id models.BlockID) *cachedMetadata {
	r.evictionLock.RLock(id.Index())
	defer r.evictionLock.RUnlock(id.Index())

	if id.EpochIndex < r.protocol.Engine().EvictionState.LastEvictedEpoch() {
		return nil
	}

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
	r.evictionLock.Lock(epochIndex)
	defer r.evictionLock.Unlock(epochIndex)

	r.cachedMetadata.Evict(epochIndex)
}

func (r *Retainer) createStorableBlockMetadata(epochIndex epoch.Index) (metas []*BlockMetadata) {
	r.evictionLock.RLock(epochIndex)
	defer r.evictionLock.RUnlock(epochIndex)

	storage := r.cachedMetadata.Get(epochIndex)
	if storage == nil {
		return metas
	}

	metas = make([]*BlockMetadata, 0, storage.Size())
	storage.ForEach(func(blockID models.BlockID, cm *cachedMetadata) bool {
		blockMetadata := newBlockMetadata(cm)
		if cm.Booker != nil {
			blockMetadata.M.ConflictIDs = r.protocol.Engine().Tangle.BlockConflicts(cm.Booker.Block)
		} else {
			blockMetadata.M.ConflictIDs = utxo.NewTransactionIDs()
		}

		metas = append(metas, blockMetadata)
		return true
	})

	return metas
}

func (r *Retainer) storeBlockMetadata(metas []*BlockMetadata) {
	for _, meta := range metas {
		if err := r.blockStorage.Set(meta.ID(), *meta); err != nil {
			panic(errors.Wrapf(err, "could not save %s to block storage", meta.ID()))
		}
	}
}

func (r *Retainer) blockMetadataFromCache(blockID models.BlockID) (storageExists bool, metadata *BlockMetadata, exists bool) {
	r.evictionLock.RLock(blockID.Index())
	defer r.evictionLock.RUnlock(blockID.Index())

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
