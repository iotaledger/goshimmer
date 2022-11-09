package virtualvoting

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region VirtualVoting ////////////////////////////////////////////////////////////////////////////////////////////////

type VirtualVoting struct {
	Events       *Events
	ValidatorSet *validator.Set

	blocks          *memstorage.EpochStorage[models.BlockID, *Block]
	conflictTracker *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower]
	sequenceTracker *sequencetracker.SequenceTracker[BlockVotePower]
	evictionMutex   sync.RWMutex

	*booker.Booker
}

func New(booker *booker.Booker, validatorSet *validator.Set, opts ...options.Option[VirtualVoting]) (newVirtualVoting *VirtualVoting) {
	return options.Apply(&VirtualVoting{
		ValidatorSet: validatorSet,
		blocks:       memstorage.NewEpochStorage[models.BlockID, *Block](),
		Booker:       booker,
	}, opts, func(o *VirtualVoting) {
		o.conflictTracker = conflicttracker.NewConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower](o.Booker.Ledger.ConflictDAG, validatorSet)
		o.sequenceTracker = sequencetracker.NewSequenceTracker[BlockVotePower](validatorSet, o.Booker.Sequence, func(sequenceID markers.SequenceID) markers.Index {
			return 0
		})

		o.Events = NewEvents()
		o.Events.ConflictTracker = o.conflictTracker.Events
		o.Events.SequenceTracker = o.sequenceTracker.Events
	}, (*VirtualVoting).setupEvents)
}

func (o *VirtualVoting) Track(block *Block) {
	if o.track(block) {
		o.Events.BlockTracked.Trigger(block)
	}
}

// Block retrieves a Block with metadata from the in-memory storage of the Booker.
func (o *VirtualVoting) Block(id models.BlockID) (block *Block, exists bool) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	return o.block(id)
}

// MarkerVoters retrieves Validators supporting a given marker.
func (o *VirtualVoting) MarkerVoters(marker markers.Marker) (voters *validator.Set) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	return o.sequenceTracker.Voters(marker)
}

// ConflictVoters retrieves Validators voting for a given conflict.
func (o *VirtualVoting) ConflictVoters(conflictID utxo.TransactionID) (voters *validator.Set) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	return o.conflictTracker.Voters(conflictID)
}

func (o *VirtualVoting) setupEvents() {
	o.Booker.Events.BlockBooked.Hook(event.NewClosure(func(block *booker.Block) {
		o.Track(NewBlock(block))
	}))

	o.Booker.Events.BlockConflictAdded.Hook(event.NewClosure(func(event *booker.BlockConflictAddedEvent) {
		o.processForkedBlock(event.Block, event.ConflictID, event.ParentConflictIDs)
	}))
	o.Booker.Events.MarkerConflictAdded.Hook(event.NewClosure(func(event *booker.MarkerConflictAddedEvent) {
		o.processForkedMarker(event.Marker, event.ConflictID, event.ParentConflictIDs)
	}))

	o.EvictionState.Events.EpochEvicted.Hook(event.NewClosure(o.evictEpoch))
}

func (o *VirtualVoting) track(block *Block) (tracked bool) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	if o.EvictionState.InEvictedEpoch(block.ID()) {
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

// block retrieves the Block with given id from the mem-storage.
func (o *VirtualVoting) block(id models.BlockID) (block *Block, exists bool) {
	if o.EvictionState.IsRootBlock(id) {
		bookerBlock, _ := o.Booker.Block(id)

		return NewBlock(bookerBlock), true
	}

	storage := o.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (o *VirtualVoting) evictEpoch(epochIndex epoch.Index) {
	o.evictionMutex.Lock()
	defer o.evictionMutex.Unlock()

	o.blocks.Evict(epochIndex)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Forking logic ////////////////////////////////////////////////////////////////////////////////////////////////

// processForkedBlock updates the Conflict weight after an individually mapped Block was forked into a new Conflict.
func (o *VirtualVoting) processForkedBlock(block *booker.Block, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	votePower := NewBlockVotePower(block.ID(), block.IssuingTime())
	o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, block.IssuerID(), votePower)
}

// take everything in future cone because it was not conflicting before and move to new conflict.
func (o *VirtualVoting) processForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	for voterID, votePower := range o.sequenceTracker.VotersWithPower(marker) {
		o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, voterID, votePower)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
