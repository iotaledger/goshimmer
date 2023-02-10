package virtualvoting

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/epochtracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region VirtualVoting ////////////////////////////////////////////////////////////////////////////////////////////////

type VirtualVoting struct {
	Events     *Events
	Validators *sybilprotection.WeightedSet

	blocks          *memstorage.EpochStorage[models.BlockID, *Block]
	conflictTracker *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower]
	sequenceTracker *sequencetracker.SequenceTracker[BlockVotePower]
	epochTracker    *epochtracker.EpochTracker
	evictionMutex   *syncutils.StarvingMutex

	optsSequenceCutoffCallback func(markers.SequenceID) markers.Index
	optsEpochCutoffCallback    func() epoch.Index

	*booker.Booker
}

func New(booker *booker.Booker, validators *sybilprotection.WeightedSet, opts ...options.Option[VirtualVoting]) (newVirtualVoting *VirtualVoting) {
	return options.Apply(&VirtualVoting{
		Validators:    validators,
		blocks:        memstorage.NewEpochStorage[models.BlockID, *Block](),
		Booker:        booker,
		evictionMutex: syncutils.NewStarvingMutex(),
		optsSequenceCutoffCallback: func(sequenceID markers.SequenceID) markers.Index {
			return 1
		},
		optsEpochCutoffCallback: func() epoch.Index {
			return 0
		},
	}, opts, func(o *VirtualVoting) {
		o.conflictTracker = conflicttracker.NewConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower](o.Booker.Ledger.ConflictDAG, validators)
		o.sequenceTracker = sequencetracker.NewSequenceTracker[BlockVotePower](validators, o.Booker.Sequence, o.optsSequenceCutoffCallback)
		o.epochTracker = epochtracker.NewEpochTracker(o.optsEpochCutoffCallback)

		o.Events = NewEvents()
		o.Events.ConflictTracker = o.conflictTracker.Events
		o.Events.SequenceTracker = o.sequenceTracker.Events
		o.Events.EpochTracker = o.epochTracker.Events
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

// MarkerVotersTotalWeight retrieves Validators supporting a given marker.
func (o *VirtualVoting) MarkerVotersTotalWeight(marker markers.Marker) (totalWeight int64) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	_ = o.sequenceTracker.Voters(marker).ForEach(func(id identity.ID) error {
		if weight, exists := o.Validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})

	return totalWeight
}

// EpochVotersTotalWeight retrieves the total weight of the Validators voting for a given epoch.
func (o *VirtualVoting) EpochVotersTotalWeight(epochIndex epoch.Index) (totalWeight int64) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	_ = o.epochTracker.Voters(epochIndex).ForEach(func(id identity.ID) error {
		if weight, exists := o.Validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})

	return totalWeight
}

// ConflictVoters retrieves Validators voting for a given conflict.
func (o *VirtualVoting) ConflictVoters(conflictID utxo.TransactionID) (voters *sybilprotection.WeightedSet) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	return o.Validators.Weights.NewWeightedSet(o.conflictTracker.Voters(conflictID).Slice()...)
}

// ConflictVotersTotalWeight retrieves the total weight of the Validators voting for a given conflict.
func (o *VirtualVoting) ConflictVotersTotalWeight(conflictID utxo.TransactionID) (totalWeight int64) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	_ = o.conflictTracker.Voters(conflictID).ForEach(func(id identity.ID) error {
		if weight, exists := o.Validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})
	return totalWeight
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
	o.Booker.Events.MarkerManager.SequenceEvicted.Attach(event.NewClosure(o.evictSequence))
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
	blockConflicts := o.Booker.BlockConflicts(block.Block)

	if _, invalid := o.conflictTracker.TrackVote(blockConflicts, block.IssuerID(), votePower); invalid {
		block.SetSubjectivelyInvalid(true)
		return true
	}
	o.sequenceTracker.TrackVotes(block.StructureDetails().PastMarkers(), block.IssuerID(), votePower)

	o.epochTracker.TrackVotes(block.Commitment().Index(), block.IssuerID(), epochtracker.EpochVotePower{Index: block.ID().Index()})

	return true
}

// block retrieves the Block with given id from the mem-storage.
func (o *VirtualVoting) block(id models.BlockID) (block *Block, exists bool) {
	if o.EvictionState.IsRootBlock(id) {
		return NewRootBlock(id), true
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

func (o *VirtualVoting) evictSequence(sequenceID markers.SequenceID) {
	o.evictionMutex.Lock()
	defer o.evictionMutex.Unlock()

	o.sequenceTracker.EvictSequence(sequenceID)
}

func (o *VirtualVoting) EvictEpochTracker(epochIndex epoch.Index) {
	o.evictionMutex.Lock()
	defer o.evictionMutex.Unlock()

	o.epochTracker.EvictEpoch(epochIndex)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Forking logic ////////////////////////////////////////////////////////////////////////////////////////////////

// processForkedBlock updates the Conflict weight after an individually mapped Block was forked into a new Conflict.
func (o *VirtualVoting) processForkedBlock(bookerBlock *booker.Block, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	votePower := NewBlockVotePower(bookerBlock.ID(), bookerBlock.IssuingTime())

	block, exists := o.Block(bookerBlock.ID())
	if !exists || block.IsSubjectivelyInvalid() {
		return
	}

	o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, block.IssuerID(), votePower)
}

// take everything in future cone because it was not conflicting before and move to new conflict.
func (o *VirtualVoting) processForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	for voterID, votePower := range o.sequenceTracker.VotersWithPower(marker) {
		o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, voterID, votePower)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEpochCutoffCallback(epochCutoffCallback func() epoch.Index) options.Option[VirtualVoting] {
	return func(virtualVoting *VirtualVoting) {
		virtualVoting.optsEpochCutoffCallback = epochCutoffCallback
	}
}

func WithSequenceCutoffCallback(sequenceCutoffCallback func(id markers.SequenceID) markers.Index) options.Option[VirtualVoting] {
	return func(virtualVoting *VirtualVoting) {
		virtualVoting.optsSequenceCutoffCallback = sequenceCutoffCallback
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
