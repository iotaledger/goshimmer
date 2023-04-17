package retainer

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const (
	prefixBlockMetadataStorage byte = iota

	prefixCommitmentDetailsStorage
)

type Retainer struct {
	Workers *workerpool.Group

	blockWorkerPool      *workerpool.WorkerPool
	commitmentWorkerPool *workerpool.WorkerPool
	cachedMetadata       *memstorage.SlotStorage[models.BlockID, *cachedMetadata]
	blockStorage         *database.PersistentSlotStorage[models.BlockID, BlockMetadata, *models.BlockID, *BlockMetadata]
	commitmentStorage    *database.PersistentSlotStorage[commitment.ID, CommitmentDetails, *commitment.ID, *CommitmentDetails]

	dbManager            *database.Manager
	protocol             *protocol.Protocol
	metadataEvictionLock *syncutils.DAGMutex[slot.Index]

	optsRealm kvstore.Realm
}

func NewRetainer(workers *workerpool.Group, protocol *protocol.Protocol, dbManager *database.Manager, opts ...options.Option[Retainer]) (r *Retainer) {
	return options.Apply(&Retainer{
		Workers:              workers,
		blockWorkerPool:      workers.CreatePool("RetainerBlock", 2),
		commitmentWorkerPool: workers.CreatePool("RetainerCommitment", 1),
		cachedMetadata:       memstorage.NewSlotStorage[models.BlockID, *cachedMetadata](),
		protocol:             protocol,
		dbManager:            dbManager,
		optsRealm:            []byte("retainer"),
	}, opts, func(r *Retainer) {
		r.blockStorage = database.NewPersistentSlotStorage[models.BlockID, BlockMetadata](dbManager, append(r.optsRealm, []byte{prefixBlockMetadataStorage}...))
		r.commitmentStorage = database.NewPersistentSlotStorage[commitment.ID, CommitmentDetails](dbManager, append(r.optsRealm, []byte{prefixCommitmentDetailsStorage}...))
		r.metadataEvictionLock = syncutils.NewDAGMutex[slot.Index]()
	}, (*Retainer).setupEvents)
}

func (r *Retainer) Shutdown() {
	r.Workers.Shutdown()
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

		if metadata.M.Accepted && !metadata.M.Confirmed && blockID.Index() <= r.protocol.Engine().LastConfirmedSlot() {
			metadata.M.ConfirmedBySlot = true
			metadata.M.ConfirmedBySlotTime = r.protocol.SlotTimeProvider().EndTime(blockID.Index())
		}
	}

	return metadata, exists
}

func (r *Retainer) Commitment(index slot.Index) (c *CommitmentDetails, exists bool) {
	return r.getCommitmentDetails(index)
}

func (r *Retainer) CommitmentByID(id commitment.ID) (c *CommitmentDetails, exists bool) {
	return r.getCommitmentDetails(id.Index())
}

func (r *Retainer) LoadAllBlockMetadata(index slot.Index) (ids *advancedset.AdvancedSet[*BlockMetadata]) {
	r.metadataEvictionLock.RLock(index)
	defer r.metadataEvictionLock.RUnlock(index)

	ids = advancedset.New[*BlockMetadata]()
	r.StreamBlocksMetadata(index, func(id models.BlockID, metadata *BlockMetadata) {
		ids.Add(metadata)
	})
	return
}

func (r *Retainer) StreamBlocksMetadata(index slot.Index, callback func(id models.BlockID, metadata *BlockMetadata)) {
	r.metadataEvictionLock.RLock(index)
	defer r.metadataEvictionLock.RUnlock(index)

	if slotStorage := r.cachedMetadata.Get(index, false); slotStorage != nil {
		slotStorage.ForEach(func(id models.BlockID, cachedMetadata *cachedMetadata) bool {
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

// BlockWorkerPool returns the block worker pool of the retainer.
func (r *Retainer) BlockWorkerPool() *workerpool.WorkerPool {
	return r.blockWorkerPool
}

// PruneUntilSlot prunes storage slots less than and equal to the given index.
func (r *Retainer) PruneUntilSlot(index slot.Index) {
	r.dbManager.PruneUntilSlot(index)
}

func (r *Retainer) setupEvents() {
	r.protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBlockDAGBlock(block)
		}
	}, event.WithWorkerPool(r.blockWorkerPool))

	// TODO: missing blocks make the node fail due to empty strong parents
	// r.protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.AttachWithWorkerPool(event.NewClosure(func(block *blockdag.Block) {
	//	cm := r.createOrGetCachedMetadata(block.ID())
	//	cm.setBlockDAGBlock(block)
	// }))

	r.protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Hook(func(block *blockdag.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBlockDAGBlock(block)
		}
	}, event.WithWorkerPool(r.blockWorkerPool))

	r.protocol.Events.Engine.Tangle.Booker.BlockBooked.Hook(func(evt *booker.BlockBookedEvent) {
		if cm := r.createOrGetCachedMetadata(evt.Block.ID()); cm != nil {
			cm.setBookerBlock(evt.Block)
			cm.Lock()
			cm.ConflictIDs = evt.ConflictIDs
			cm.Unlock()
		}
	}, event.WithWorkerPool(r.blockWorkerPool))

	r.protocol.Events.Engine.Tangle.Booker.BlockTracked.Hook(func(block *booker.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setVirtualVotingBlock(block)
		}
	}, event.WithWorkerPool(r.blockWorkerPool))

	congestionControl := func(block *scheduler.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setSchedulerBlock(block)
		}
	}
	r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Hook(congestionControl, event.WithWorkerPool(r.blockWorkerPool))
	r.protocol.Events.CongestionControl.Scheduler.BlockDropped.Hook(congestionControl, event.WithWorkerPool(r.blockWorkerPool))
	r.protocol.Events.CongestionControl.Scheduler.BlockSkipped.Hook(congestionControl, event.WithWorkerPool(r.blockWorkerPool))

	r.protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setAcceptanceBlock(block)
	}, event.WithWorkerPool(r.blockWorkerPool))

	r.protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setConfirmationBlock(block)
		}
	}, event.WithWorkerPool(r.blockWorkerPool))

	r.protocol.Events.Engine.EvictionState.SlotEvicted.Hook(r.storeAndEvictSlot, event.WithWorkerPool(r.blockWorkerPool))

	r.protocol.Events.Engine.Notarization.SlotCommitted.Hook(func(e *notarization.SlotCommittedDetails) {
		if e.Commitment.Index() < r.protocol.Engine().EvictionState.LastEvictedSlot() {
			return
		}

		cd := newCommitmentDetails()
		cd.setCommitment(e)
		r.storeCommitmentDetails(cd)
	}, event.WithWorkerPool(r.commitmentWorkerPool))
}

func (r *Retainer) createOrGetCachedMetadata(id models.BlockID) *cachedMetadata {
	r.metadataEvictionLock.RLock(id.Index())
	defer r.metadataEvictionLock.RUnlock(id.Index())

	if id.SlotIndex < r.protocol.Engine().EvictionState.LastEvictedSlot() {
		return nil
	}

	storage := r.cachedMetadata.Get(id.Index(), true)
	cm, _ := storage.GetOrCreate(id, newCachedMetadata)
	return cm
}

func (r *Retainer) storeAndEvictSlot(index slot.Index) {
	// First we read the data from storage.
	metas := r.createStorableBlockMetadata(index)

	// Now we store it to disk (slow).
	r.storeBlockMetadata(metas)

	// Once everything is stored to disk, we evict it from cache.
	// Therefore, we make sure that we can always first try to read BlockMetadata from cache and if it's not in cache
	// anymore it is already written to disk.
	r.metadataEvictionLock.Lock(index)
	r.cachedMetadata.Evict(index)
	r.metadataEvictionLock.Unlock(index)
}

func (r *Retainer) createStorableBlockMetadata(index slot.Index) (metas []*BlockMetadata) {
	r.metadataEvictionLock.RLock(index)
	defer r.metadataEvictionLock.RUnlock(index)

	storage := r.cachedMetadata.Get(index)
	if storage == nil {
		return metas
	}

	metas = make([]*BlockMetadata, 0, storage.Size())
	storage.ForEach(func(blockID models.BlockID, cm *cachedMetadata) bool {
		blockMetadata := newBlockMetadata(cm)
		if cm.Booker != nil {
			blockMetadata.M.ConflictIDs = r.protocol.Engine().Tangle.Booker().BlockConflicts(cm.Booker.Block)
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

func (r *Retainer) storeCommitmentDetails(c *CommitmentDetails) {
	if err := r.commitmentStorage.Set(c.ID(), *c); err != nil {
		panic(errors.Wrapf(err, "could not save %s to commitment storage", c.ID()))
	}
}

func (r *Retainer) blockMetadataFromCache(blockID models.BlockID) (storageExists bool, metadata *BlockMetadata, exists bool) {
	r.metadataEvictionLock.RLock(blockID.Index())
	defer r.metadataEvictionLock.RUnlock(blockID.Index())

	storage := r.cachedMetadata.Get(blockID.Index())
	if storage == nil {
		return false, nil, false
	}

	cm, exists := storage.Get(blockID)
	return true, newBlockMetadata(cm), exists
}

func (r *Retainer) getCommitmentDetails(index slot.Index) (c *CommitmentDetails, exists bool) {
	if index < 0 {
		return
	}

	// get from persistent storage
	c = newCommitmentDetails()
	err := r.commitmentStorage.Iterate(index, func(key commitment.ID, value CommitmentDetails) (advance bool) {
		*c = value
		c.SetID(c.M.ID)
		exists = true

		return false
	})
	if err != nil {
		return nil, false
	}

	return c, exists
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRealm(realm kvstore.Realm) options.Option[Retainer] {
	return func(r *Retainer) {
		r.optsRealm = realm
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
