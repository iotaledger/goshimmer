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
	metadata *memstorage.EpochStorage[models.BlockID, *Metadata]

	dbManager       *database.Manager
	evictionManager *eviction.LockableManager[models.BlockID]
}

func NewRetainer(dbManager *database.Manager, evictionManager *eviction.Manager[models.BlockID]) (r *Retainer) {
	r = &Retainer{
		dbManager:       dbManager,
		metadata:        memstorage.NewEpochStorage[models.BlockID, *Metadata](),
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
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	// TODO: store metadata to disk and evict
	//  should we use object storage or just plain KV store?
}

// TODO: provide functionality to retrieve data from retainer
//  metadata needs to be a storable and serializable -> potentially use serix JSON generation and get rid (partially) of jsonmodels?
