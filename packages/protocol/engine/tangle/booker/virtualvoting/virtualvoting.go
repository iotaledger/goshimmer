package virtualvoting

import (
	"fmt"
	"math"

	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/slottracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region VirtualVoting ////////////////////////////////////////////////////////////////////////////////////////////////

type VirtualVoting struct {
	Events          *Events
	Validators      *sybilprotection.WeightedSet
	ConflictDAG     *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]
	SequenceManager *markers.SequenceManager

	conflictTracker *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower]
	sequenceTracker *sequencetracker.SequenceTracker[BlockVotePower]
	slotTracker     *slottracker.SlotTracker
	evictionMutex   *syncutils.StarvingMutex

	optsSequenceCutoffCallback func(markers.SequenceID) markers.Index
	optsSlotCutoffCallback     func() slot.Index

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

		optsSlotCutoffCallback: func() slot.Index {
			return 0
		},
	}, opts, func(o *VirtualVoting) {
		o.conflictTracker = conflicttracker.NewConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower](conflictDAG, validators)
		o.sequenceTracker = sequencetracker.NewSequenceTracker[BlockVotePower](validators, sequenceManager.Sequence, o.optsSequenceCutoffCallback)
		o.slotTracker = slottracker.NewSlotTracker(o.optsSlotCutoffCallback)

		o.Events = NewEvents()
		o.Events.ConflictTracker = o.conflictTracker.Events
		o.Events.SequenceTracker = o.sequenceTracker.Events
		o.Events.SlotTracker = o.slotTracker.Events
	})
}

func (o *VirtualVoting) Track(block *Block, conflictIDs utxo.TransactionIDs) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	votePower := NewBlockVotePower(block.ID(), block.IssuingTime())

	if _, invalid := o.conflictTracker.TrackVote(conflictIDs, block.IssuerID(), votePower); invalid {
		fmt.Println("block is subjectively invalid", block.ID())
		block.SetSubjectivelyInvalid(true)
	} else {
		o.sequenceTracker.TrackVotes(block.StructureDetails().PastMarkers(), block.IssuerID(), votePower)
		o.slotTracker.TrackVotes(block.Commitment().Index(), block.IssuerID(), slottracker.SlotVotePower{Index: block.ID().Index()})
	}

	o.Events.BlockTracked.Trigger(block)
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

// SlotVotersTotalWeight retrieves the total weight of the Validators voting for a given slot.
func (o *VirtualVoting) SlotVotersTotalWeight(slotIndex slot.Index) (totalWeight int64) {
	o.evictionMutex.RLock()
	defer o.evictionMutex.RUnlock()

	_ = o.slotTracker.Voters(slotIndex).ForEach(func(id identity.ID) error {
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

	if conflict, exists := o.ConflictDAG.Conflict(conflictID); exists {
		if conflict.ConfirmationState().IsAccepted() {
			return math.MaxInt64
		} else if conflict.ConfirmationState().IsRejected() {
			return 0
		}
	}

	_ = o.conflictTracker.Voters(conflictID).ForEach(func(id identity.ID) error {
		if weight, exists := o.Validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})
	return totalWeight
}

func (o *VirtualVoting) EvictSequence(sequenceID markers.SequenceID) {
	o.evictionMutex.Lock()
	defer o.evictionMutex.Unlock()

	o.sequenceTracker.EvictSequence(sequenceID)
}

func (o *VirtualVoting) EvictSlotTracker(slotIndex slot.Index) {
	o.evictionMutex.Lock()
	defer o.evictionMutex.Unlock()

	o.slotTracker.EvictSlot(slotIndex)
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

func (o *VirtualVoting) ProcessForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	// take everything in future cone because it was not conflicting before and move to new conflict.
	for voterID, votePower := range o.sequenceTracker.VotersWithPower(marker) {
		o.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, voterID, votePower)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSlotCutoffCallback(slotCutoffCallback func() slot.Index) options.Option[VirtualVoting] {
	return func(virtualVoting *VirtualVoting) {
		virtualVoting.optsSlotCutoffCallback = slotCutoffCallback
	}
}

func WithSequenceCutoffCallback(sequenceCutoffCallback func(id markers.SequenceID) markers.Index) options.Option[VirtualVoting] {
	return func(virtualVoting *VirtualVoting) {
		virtualVoting.optsSequenceCutoffCallback = sequenceCutoffCallback
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
