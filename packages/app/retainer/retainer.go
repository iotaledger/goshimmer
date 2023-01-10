package retainer

import (
	"sync"

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
	cachedMetadata *memstorage.EpochStorage[models.BlockID, *cachedMetadata]
	blockStorage   *database.PersistentEpochStorage[models.BlockID, BlockMetadata, *models.BlockID, *BlockMetadata]

	dbManager    *database.Manager
	protocol     *protocol.Protocol
	evictionLock sync.RWMutex

	optsRealm kvstore.Realm
}

func NewRetainer(protocol *protocol.Protocol, dbManager *database.Manager, opts ...options.Option[Retainer]) (r *Retainer) {
	return options.Apply(&Retainer{
		cachedMetadata: memstorage.NewEpochStorage[models.BlockID, *cachedMetadata](),
		protocol:       protocol,
		dbManager:      dbManager,
		optsRealm:      []byte("retainer"),
	}, opts, (*Retainer).setupEvents, func(r *Retainer) {
		r.blockStorage = database.NewPersistentEpochStorage[models.BlockID, BlockMetadata](dbManager, r.optsRealm)
	})
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

	metadata, exists = r.blockStorage.Get(blockID)
	if exists {
		metadata.SetID(metadata.M.Id)

		if metadata.M.Accepted && !metadata.M.Confirmed && blockID.Index() <= r.protocol.Engine().LastConfirmedEpoch() {
			metadata.M.ConfirmedByEpoch = true
			metadata.M.ConfirmedByEpochTime = blockID.Index().EndTime()
		}
	}

	return metadata, exists
}

func (r *Retainer) LoadAll(index epoch.Index) (ids *set.AdvancedSet[*BlockMetadata]) {
	ids = set.NewAdvancedSet[*BlockMetadata]()
	r.Stream(index, func(id models.BlockID, metadata *BlockMetadata) {
		ids.Add(metadata)
	})
	return
}

func (r *Retainer) Stream(index epoch.Index, callback func(id models.BlockID, metadata *BlockMetadata)) {
	if epochStorage := r.cachedMetadata.Get(index, false); epochStorage != nil {
		epochStorage.ForEach(func(id models.BlockID, cachedMetadata *cachedMetadata) bool {
			callback(id, newBlockMetadata(cachedMetadata))
			return true
		})
		return
	}

	r.blockStorage.Iterate(index, func(id models.BlockID, metadata *BlockMetadata) bool {
		callback(id, metadata)
		return true
	})
}

// DatabaseSize returns the size of the underlying databases.
func (r *Retainer) DatabaseSize() int64 {
	return r.dbManager.TotalStorageSize()
}

// PruneUntilEpoch prunes storage epochs less than and equal to the given index.
func (r *Retainer) PruneUntilEpoch(epochIndex epoch.Index) {
	r.dbManager.PruneUntilEpoch(epochIndex)
}

func (r *Retainer) setupEvents() {
	r.protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlockDAGBlock(block)
	}))

	// TODO: missing blocks make the node fail due to empty strong parents
	//r.protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(func(block *blockdag.Block) {
	//	cm := r.createOrGetCachedMetadata(block.ID())
	//	cm.setBlockDAGBlock(block)
	//}))

	r.protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		//fmt.Println("BlockSolid", block.ID(), "issuer", block.IssuerID())
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setBlockDAGBlock(block)
	}))

	r.protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		//fmt.Println("BlockBooked", block.ID(), "issuer", block.IssuerID())
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

	//r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
	//	fmt.Println("BlockScheduled", block.ID(), "issuer", block.IssuerID())
	//}))
	//r.protocol.Events.CongestionControl.Scheduler.BlockDropped.Attach(event.NewClosure(func(block *scheduler.Block) {
	//	fmt.Println("BlockDropped", block.ID(), "issuer", block.IssuerID())
	//}))
	//r.protocol.Events.CongestionControl.Scheduler.BlockSkipped.Attach(event.NewClosure(func(block *scheduler.Block) {
	//	fmt.Println("BlockSkipped", block.ID(), "issuer", block.IssuerID())
	//}))
	r.protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		//fmt.Println("Block accepted", block.ID(), "issuer", block.IssuerID())
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setAcceptanceBlock(block)
	}))

	r.protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Attach(event.NewClosure(func(block *blockgadget.Block) {
		cm := r.createOrGetCachedMetadata(block.ID())
		cm.setConfirmationBlock(block)
	}))

	r.protocol.Events.Engine.EvictionState.EpochEvicted.Hook(event.NewClosure(r.storeAndEvictEpoch))
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
	r.cachedMetadata.Evict(epochIndex)
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
		if err := r.blockStorage.Set(meta.ID(), meta); err != nil {
			panic(errors.Wrapf(err, "could not save %s to block storage", meta.ID()))
		}
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
