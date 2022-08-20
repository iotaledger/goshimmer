package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/booker"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region OnTangleVoting ///////////////////////////////////////////////////////////////////////////////////////////////

type OnTangleVoting struct {
	Events *Events

	blocks          *memstorage.EpochStorage[models.BlockID, *Block]
	evictionManager *eviction.LockableManager[models.BlockID]

	optsBooker []options.Option[booker.Booker]

	*booker.Booker
}

func New(evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[OnTangleVoting]) (otv *OnTangleVoting) {
	otv = options.Apply(&OnTangleVoting{
		Events:          newEvents(),
		blocks:          memstorage.NewEpochStorage[models.BlockID, *Block](),
		evictionManager: evictionManager.Lockable(),
		optsBooker:      make([]options.Option[booker.Booker], 0),
	}, opts)
	otv.Booker = booker.New(evictionManager, otv.optsBooker...)

	otv.Booker.Events.BlockBooked.Hook(event.NewClosure(func(block *booker.Block) {
		otv.Track(NewBlock(block))
	}))

	otv.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(otv.evictEpoch))

	return otv
}

func (o *OnTangleVoting) evictEpoch(epochIndex epoch.Index) {
	o.evictionManager.Lock()
	defer o.evictionManager.Unlock()

	o.blocks.EvictEpoch(epochIndex)
}

func (o *OnTangleVoting) Track(block *Block) {
	o.evictionManager.RLock()
	defer o.evictionManager.RUnlock()

	if o.evictionManager.IsTooOld(block.ID()) {
		return
	}

	o.blocks.Get(block.ID().Index(), true).Set(block.ID(), block)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBookerOptions(opts ...options.Option[booker.Booker]) options.Option[OnTangleVoting] {
	return func(b *OnTangleVoting) {
		b.optsBooker = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
