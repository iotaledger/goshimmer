package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/booker"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

// region OnTangleVoting ///////////////////////////////////////////////////////////////////////////////////////////////

type OnTangleVoting struct {
	Events *Events

	blocks          *memstorage.EpochStorage[models.BlockID, *Block]
	validatorSet    *validator.Set
	conflictTracker *votes.ConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower]
	sequenceTracker *votes.SequenceTracker[BlockVotePower]
	evictionManager *eviction.LockableManager[models.BlockID]

	optsBooker []options.Option[booker.Booker]

	*booker.Booker
}

func New(validatorSet *validator.Set, evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[OnTangleVoting]) (otv *OnTangleVoting) {
	otv = options.Apply(&OnTangleVoting{
		Events:          newEvents(),
		blocks:          memstorage.NewEpochStorage[models.BlockID, *Block](),
		validatorSet:    validatorSet,
		evictionManager: evictionManager.Lockable(),
		optsBooker:      make([]options.Option[booker.Booker], 0),
	}, opts)
	otv.Booker = booker.New(evictionManager, otv.optsBooker...)
	otv.conflictTracker = votes.NewConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower](otv.Booker.Ledger.ConflictDAG, validatorSet)
	otv.sequenceTracker = votes.NewSequenceTracker[BlockVotePower](validatorSet, otv.Booker.Sequence, func(sequenceID markers.SequenceID) markers.Index {
		return 0
	})

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
	if o.track(block) {
		o.Events.BlockTracked.Trigger(block)
	}
}

func (o *OnTangleVoting) track(block *Block) (tracked bool) {
	o.evictionManager.RLock()
	defer o.evictionManager.RUnlock()

	if o.evictionManager.IsTooOld(block.ID()) {
		return false
	}

	o.blocks.Get(block.ID().Index(), true).Set(block.ID(), block)

	votePower := NewBlockVotePower(block.ID(), block.IssuingTime())
	if _, invalid := o.conflictTracker.TrackVote(o.Booker.BlockConflicts(block.Block), block.IssuerID(), votePower); invalid {

		return false
	}

	o.sequenceTracker.TrackVotes(block.StructureDetails().PastMarkers(), block.IssuerID(), votePower)

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBookerOptions(opts ...options.Option[booker.Booker]) options.Option[OnTangleVoting] {
	return func(b *OnTangleVoting) {
		b.optsBooker = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
