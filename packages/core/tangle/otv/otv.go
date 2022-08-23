package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	booker2 "github.com/iotaledger/goshimmer/packages/core/tangle/booker"
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

	optsBooker []options.Option[booker2.Booker]

	*booker2.Booker
}

func New(validatorSet *validator.Set, evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[OnTangleVoting]) (otv *OnTangleVoting) {
	otv = options.Apply(&OnTangleVoting{
		blocks:          memstorage.NewEpochStorage[models.BlockID, *Block](),
		validatorSet:    validatorSet,
		evictionManager: evictionManager.Lockable(),
		optsBooker:      make([]options.Option[booker2.Booker], 0),
	}, opts)
	otv.Booker = booker2.New(evictionManager, otv.optsBooker...)
	otv.conflictTracker = votes.NewConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower](otv.Booker.Ledger.ConflictDAG, validatorSet)
	otv.sequenceTracker = votes.NewSequenceTracker[BlockVotePower](validatorSet, otv.Booker.Sequence, func(sequenceID markers.SequenceID) markers.Index {
		return 0
	})
	otv.Events = newEvents(otv.conflictTracker.Events, otv.sequenceTracker.Events)

	otv.Booker.Events.BlockBooked.Hook(event.NewClosure(func(block *booker2.Block) {
		otv.Track(NewBlock(block))
	}))

	otv.Booker.Events.BlockConflictAdded.Hook(event.NewClosure(func(event *booker2.BlockConflictAddedEvent) {
		otv.processForkedBlock(event.Block, event.ConflictID, event.ParentConflictIDs)
	}))
	otv.Booker.Events.MarkerConflictAdded.Hook(event.NewClosure(func(event *booker2.MarkerConflictAddedEvent) {
		otv.processForkedMarker(event.Marker, event.ConflictID, event.ParentConflictIDs)
	}))

	otv.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(otv.evictEpoch))

	return otv
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

func (o *OnTangleVoting) evictEpoch(epochIndex epoch.Index) {
	o.evictionManager.Lock()
	defer o.evictionManager.Unlock()

	o.blocks.EvictEpoch(epochIndex)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Forking logic ////////////////////////////////////////////////////////////////////////////////////////////////

// processForkedBlock updates the Conflict weight after an individually mapped Block was forked into a new Conflict.
func (o *OnTangleVoting) processForkedBlock(block *booker2.Block, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	votePower := NewBlockVotePower(block.ID(), block.IssuingTime())
	o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, block.IssuerID(), votePower)
}

// take everything in future cone because it was not conflicting before and move to new conflict.
func (o *OnTangleVoting) processForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	for voterID, votePower := range o.sequenceTracker.VotersWithPower(marker) {
		o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, voterID, votePower)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBookerOptions(opts ...options.Option[booker2.Booker]) options.Option[OnTangleVoting] {
	return func(b *OnTangleVoting) {
		b.optsBooker = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
