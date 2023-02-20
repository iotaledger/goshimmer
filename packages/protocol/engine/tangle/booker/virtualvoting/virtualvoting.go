package virtualvoting

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/epochtracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hive.go/core/workerpool"
)

// region VirtualVoting ////////////////////////////////////////////////////////////////////////////////////////////////

type VirtualVoting struct {
	Events          *Events
	Validators      *sybilprotection.WeightedSet
	ConflictDAG     *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]
	SequenceManager *markers.SequenceManager

	conflictTracker *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower]
	sequenceTracker *sequencetracker.SequenceTracker[BlockVotePower]
	epochTracker    *epochtracker.EpochTracker
	evictionMutex   *syncutils.StarvingMutex

	optsSequenceCutoffCallback func(markers.SequenceID) markers.Index
	optsEpochCutoffCallback    func() epoch.Index

	Workers *workerpool.Group
}

func New(workers *workerpool.Group, conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], sequenceManager *markers.SequenceManager, validators *sybilprotection.WeightedSet, opts ...options.Option[VirtualVoting]) (newVirtualVoting *VirtualVoting) {
	return options.Apply(&VirtualVoting{
		Validators:      validators,
		Workers:         workers,
		ConflictDAG:     conflictDAG,
		SequenceManager: sequenceManager,
		evictionMutex:   syncutils.NewStarvingMutex(),
		optsSequenceCutoffCallback: func(sequenceID markers.SequenceID) markers.Index {
			return 1
		},

		optsEpochCutoffCallback: func() epoch.Index {
			return 0
		},
	}, opts, func(o *VirtualVoting) {
		o.conflictTracker = conflicttracker.NewConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower](conflictDAG, validators)
		o.sequenceTracker = sequencetracker.NewSequenceTracker[BlockVotePower](validators, sequenceManager.Sequence, o.optsSequenceCutoffCallback)
		o.epochTracker = epochtracker.NewEpochTracker(o.optsEpochCutoffCallback)

		o.Events = NewEvents()
		o.Events.ConflictTracker = o.conflictTracker.Events
		o.Events.SequenceTracker = o.sequenceTracker.Events
		o.Events.EpochTracker = o.epochTracker.Events
	})
}

func (o *VirtualVoting) Track(block *Block, conflictIDs utxo.TransactionIDs) {
	if o.track(block, conflictIDs) {
		o.Events.BlockTracked.Trigger(block)
	}
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
	/*
		event.Hook(o.Booker.Events.BlockBooked, func(evt *booker.BlockBookedEvent) {
			o.Track(NewBlock(evt.Block), evt.ConflictIDs)
		})
		event.Hook(o.Booker.Events.BlockConflictAdded, func(event *booker.BlockConflictAddedEvent) {
			o.processForkedBlock(event.Block, event.ConflictID, event.ParentConflictIDs)
		})
		event.Hook(o.Booker.Events.MarkerConflictAdded, func(event *booker.MarkerConflictAddedEvent) {
			o.processForkedMarker(event.Marker, event.ConflictID, event.ParentConflictIDs)
		})
	*/
	/*
	wp := o.Workers.CreatePool("Eviction", 1) // Using just 1 worker to avoid contention
	event.AttachWithWorkerPool(o.Booker.Events.MarkerManager.SequenceEvicted, o.evictSequence, wp)
	event.Hook(o.BlockDAG.EvictionState.Events.EpochEvicted, o.evictEpoch)
	*/
}

func (o *VirtualVoting) track(block *Block, conflictIDs utxo.TransactionIDs) (tracked bool) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	// TODO: this check should happen outside.
	/*
		if o.BlockDAG.EvictionState.InEvictedEpoch(block.ID()) {
			return false
		}
	*/

	votePower := NewBlockVotePower(block.ID(), block.IssuingTime())

	if _, invalid := o.conflictTracker.TrackVote(conflictIDs, block.IssuerID(), votePower); invalid {
		block.SetSubjectivelyInvalid(true)
		return true
	}
	o.sequenceTracker.TrackVotes(block.StructureDetails().PastMarkers(), block.IssuerID(), votePower)

	o.epochTracker.TrackVotes(block.Commitment().Index(), block.IssuerID(), epochtracker.EpochVotePower{Index: block.ID().Index()})

	return true
}

func (o *VirtualVoting) EvictSequence(sequenceID markers.SequenceID) {
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

// ProcessForkedBlock updates the Conflict weight after an individually mapped Block was forked into a new Conflict.
func (o *VirtualVoting) ProcessForkedBlock(block *Block, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	votePower := NewBlockVotePower(block.ID(), block.IssuingTime())

	// Do not apply votes of subjectively invalid blocks on forking. Votes of subjectively invalid blocks are also not counted
	// when booking.
	if block.IsSubjectivelyInvalid() {
		return
	}

	o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, block.IssuerID(), votePower)
}

// take everything in future cone because it was not conflicting before and move to new conflict.
func (o *VirtualVoting) ProcessForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
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
