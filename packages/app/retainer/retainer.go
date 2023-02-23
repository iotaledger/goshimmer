package retainer

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const epochInCache = 20
const (
	prefixBlockMetadataStorage byte = iota

	prefixCommitmentDetailsStorage
)

type Retainer struct {
	blockWorkerPool      *workerpool.UnboundedWorkerPool
	commitmentWorkerPool *workerpool.UnboundedWorkerPool
	cachedMetadata       *memstorage.EpochStorage[models.BlockID, *cachedMetadata]
	blockStorage         *database.PersistentEpochStorage[models.BlockID, BlockMetadata, *models.BlockID, *BlockMetadata]
	commitmentStorage    *database.PersistentEpochStorage[commitment.ID, CommitmentDetails, *commitment.ID, *CommitmentDetails]

	dbManager            *database.Manager
	protocol             *protocol.Protocol
	metadataEvictionLock *syncutils.DAGMutex[epoch.Index]

	optsRealm kvstore.Realm
}

func NewRetainer(workers *workerpool.Group, protocol *protocol.Protocol, dbManager *database.Manager, opts ...options.Option[Retainer]) (r *Retainer) {
	return options.Apply(&Retainer{
		blockWorkerPool:      workers.CreatePool("RetainerBlock", 2),
		commitmentWorkerPool: workers.CreatePool("RetainerCommitment", 1),
		cachedMetadata:       memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		protocol:             protocol,
		dbManager:            dbManager,
		optsRealm:            []byte("retainer"),
	}, opts, (*Retainer).setupEvents, func(r *Retainer) {
		r.blockStorage = database.NewPersistentEpochStorage[models.BlockID, BlockMetadata](dbManager, append(r.optsRealm, []byte{prefixBlockMetadataStorage}...))
		r.commitmentStorage = database.NewPersistentEpochStorage[commitment.ID, CommitmentDetails](dbManager, append(r.optsRealm, []byte{prefixCommitmentDetailsStorage}...))
		r.metadataEvictionLock = syncutils.NewDAGMutex[epoch.Index]()
	})
}

func (r *Retainer) Shutdown() {
	r.blockWorkerPool.Shutdown()
	r.commitmentWorkerPool.Shutdown()
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

func (r *Retainer) Commitment(index epoch.Index) (c *CommitmentDetails, exists bool) {
	return r.getCommitmentDetails(index)
}

func (r *Retainer) CommitmentbyID(id commitment.ID) (c *CommitmentDetails, exists bool) {
	return r.getCommitmentDetails(id.Index())
}

func (r *Retainer) LoadAllBlockMetadata(index epoch.Index) (ids *set.AdvancedSet[*BlockMetadata]) {
	r.metadataEvictionLock.RLock(index)
	defer r.metadataEvictionLock.RUnlock(index)

	ids = set.NewAdvancedSet[*BlockMetadata]()
	r.StreamBlocksMetadata(index, func(id models.BlockID, metadata *BlockMetadata) {
		ids.Add(metadata)
	})
	return
}

func (r *Retainer) StreamBlocksMetadata(index epoch.Index, callback func(id models.BlockID, metadata *BlockMetadata)) {
	r.metadataEvictionLock.RLock(index)
	defer r.metadataEvictionLock.RUnlock(index)

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

// BlockWorkerPool returns the block worker pool of the retainer.
func (r *Retainer) BlockWorkerPool() *workerpool.UnboundedWorkerPool {
	return r.blockWorkerPool
}

// CommitmentWorkerPool returns the commitment worker pool of the retainer.
func (r *Retainer) CommitmentWorkerPool() *workerpool.UnboundedWorkerPool {
	return r.commitmentWorkerPool
}

// PruneUntilEpoch prunes storage epochs less than and equal to the given index.
func (r *Retainer) PruneUntilEpoch(epochIndex epoch.Index) {
	r.dbManager.PruneUntilEpoch(epochIndex)
}

func (r *Retainer) setupEvents() {
	event.AttachWithWorkerPool(r.protocol.Events.Engine.Tangle.BlockDAG.BlockAttached, func(block *blockdag.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBlockDAGBlock(block)
		}
	}, r.blockWorkerPool)

	// TODO: missing blocks make the node fail due to empty strong parents
	// r.protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.AttachWithWorkerPool(event.NewClosure(func(block *blockdag.Block) {
	//	cm := r.createOrGetCachedMetadata(block.ID())
	//	cm.setBlockDAGBlock(block)
	// }))

	event.AttachWithWorkerPool(r.protocol.Events.Engine.Tangle.BlockDAG.BlockSolid, func(block *blockdag.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBlockDAGBlock(block)
		}
	}, r.blockWorkerPool)

	event.AttachWithWorkerPool(r.protocol.Events.Engine.Tangle.Booker.BlockBooked, func(block *booker.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setBookerBlock(block)
			cm.Lock()
			cm.ConflictIDs = r.protocol.Engine().Tangle.Booker.BlockConflicts(block)
			cm.Unlock()
		}
	}, r.blockWorkerPool)

	event.AttachWithWorkerPool(r.protocol.Events.Engine.Tangle.VirtualVoting.BlockTracked, func(block *virtualvoting.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setVirtualVotingBlock(block)
		}
	}, r.blockWorkerPool)

	congestionControl := func(block *scheduler.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setSchedulerBlock(block)
		}
	}
	event.AttachWithWorkerPool(r.protocol.Events.CongestionControl.Scheduler.BlockScheduled, congestionControl, r.blockWorkerPool)
	event.AttachWithWorkerPool(r.protocol.Events.CongestionControl.Scheduler.BlockDropped, congestionControl, r.blockWorkerPool)
	event.AttachWithWorkerPool(r.protocol.Events.CongestionControl.Scheduler.BlockSkipped, congestionControl, r.blockWorkerPool)

	event.AttachWithWorkerPool(r.protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted, func(block *blockgadget.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setAcceptanceBlock(block)
	}, r.blockWorkerPool)

	event.AttachWithWorkerPool(r.protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed, func(block *blockgadget.Block) {
		if cm := r.createOrGetCachedMetadata(block.ID()); cm != nil {
			cm.setConfirmationBlock(block)
		}
	}, r.blockWorkerPool)

	event.Hook(r.protocol.Events.Engine.EvictionState.EpochEvicted, r.storeAndEvictEpoch)

	r.protocol.Engine().NotarizationManager.Events.EpochCommitted.AttachWithWorkerPool(event.NewClosure(func(e *notarization.EpochCommittedDetails) {
		if e.Commitment.Index() < r.protocol.Engine().EvictionState.LastEvictedEpoch() {
			return
		}

		cd, _ := r.getCommitmentDetails(e.Commitment.Index())
		if cd != nil {
			var (
				blockIDs       = make(models.BlockIDs)
				txIDs          = utxo.NewTransactionIDs()
				spentOutputs   = utxo.NewOutputIDs()
				createdOutputs = utxo.NewOutputIDs()
			)
			_ = e.AcceptedBlocks.Stream(func(key models.BlockID) bool {
				blockIDs.Add(key)
				return true
			})
			_ = e.AcceptedTransactions.Stream(func(key utxo.TransactionID) bool {
				txIDs.Add(key)
				return true
			})

			_ = e.SpentOutputs(e.Commitment.Index(), func(owm *ledger.OutputWithMetadata) error {
				spentOutputs.Add(owm.ID())
				return nil
			})

			_ = e.CreatedOutputs(e.Commitment.Index(), func(owm *ledger.OutputWithMetadata) error {
				createdOutputs.Add(owm.ID())
				return nil
			})

			cd.setCommitment(e.Commitment, blockIDs, txIDs, createdOutputs, spentOutputs)
			r.storeCommitmentDetails(cd)
		}
	}), r.commitmentWorkerPool)
}

func (r *Retainer) createOrGetCachedMetadata(id models.BlockID) *cachedMetadata {
	r.metadataEvictionLock.RLock(id.Index())
	defer r.metadataEvictionLock.RUnlock(id.Index())

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
	r.metadataEvictionLock.Lock(epochIndex)
	r.cachedMetadata.Evict(epochIndex)
	r.metadataEvictionLock.Unlock(epochIndex)
}

func (r *Retainer) createStorableBlockMetadata(epochIndex epoch.Index) (metas []*BlockMetadata) {
	r.metadataEvictionLock.RLock(epochIndex)
	defer r.metadataEvictionLock.RUnlock(epochIndex)

	storage := r.cachedMetadata.Get(epochIndex)
	if storage == nil {
		return metas
	}

	metas = make([]*BlockMetadata, 0, storage.Size())
	storage.ForEach(func(blockID models.BlockID, cm *cachedMetadata) bool {
		blockMetadata := newBlockMetadata(cm)
		if cm.Booker != nil {
			blockMetadata.M.ConflictIDs = r.protocol.Engine().Tangle.Booker.BlockConflicts(cm.Booker.Block)
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
	if c == nil {
		return
	}

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

func (r *Retainer) getCommitmentDetails(index epoch.Index) (c *CommitmentDetails, exists bool) {
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
