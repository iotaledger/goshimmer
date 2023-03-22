package markervirtualvoting

import (
	"fmt"
	"math"

	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/slottracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region VirtualVoting ////////////////////////////////////////////////////////////////////////////////////////////////

type VirtualVoting struct {
	events          *booker.VirtualVotingEvents
	Validators      *sybilprotection.WeightedSet
	ConflictDAG     *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]
	sequenceManager *markers.SequenceManager

	conflictTracker *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, booker.BlockVotePower]
	sequenceTracker *sequencetracker.SequenceTracker[booker.BlockVotePower]
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
		sequenceManager: sequenceManager,
		evictionMutex:   syncutils.NewStarvingMutex(),
		optsSequenceCutoffCallback: func(sequenceID markers.SequenceID) markers.Index {
			return 1
		},

		optsSlotCutoffCallback: func() slot.Index {
			return 0
		},
	}, opts, func(v *VirtualVoting) {
		v.conflictTracker = conflicttracker.NewConflictTracker[utxo.TransactionID, utxo.OutputID, booker.BlockVotePower](conflictDAG, validators)
		v.sequenceTracker = sequencetracker.NewSequenceTracker[booker.BlockVotePower](validators, sequenceManager.Sequence, v.optsSequenceCutoffCallback)
		v.slotTracker = slottracker.NewSlotTracker(v.optsSlotCutoffCallback)

		v.events = booker.NewVirtualVotingEvents()
		v.events.ConflictTracker.LinkTo(v.conflictTracker.Events)
		v.events.SequenceTracker.LinkTo(v.sequenceTracker.Events)
		v.events.SlotTracker.LinkTo(v.slotTracker.Events)
	})
}

var _ booker.VirtualVoting = new(VirtualVoting)

func (v *VirtualVoting) Events() *booker.VirtualVotingEvents {
	return v.events
}

func (v *VirtualVoting) SequenceManager() *markers.SequenceManager {
	return v.sequenceManager
}

func (v *VirtualVoting) ConflictTracker() *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, booker.BlockVotePower] {
	return v.conflictTracker
}

func (v *VirtualVoting) SequenceTracker() *sequencetracker.SequenceTracker[booker.BlockVotePower] {
	return v.sequenceTracker
}

func (v *VirtualVoting) Track(block *booker.Block, conflictIDs utxo.TransactionIDs) {
	v.evictionMutex.RLock()
	defer v.evictionMutex.RUnlock()

	votePower := booker.NewBlockVotePower(block.ID(), block.IssuingTime())

	if _, invalid := v.conflictTracker.TrackVote(conflictIDs, block.IssuerID(), votePower); invalid {
		fmt.Println("block is subjectively invalid", block.ID())
		block.SetSubjectivelyInvalid(true)
	} else {
		v.sequenceTracker.TrackVotes(block.StructureDetails().PastMarkers(), block.IssuerID(), votePower)
		v.slotTracker.TrackVotes(block.Commitment().Index(), block.IssuerID(), slottracker.SlotVotePower{Index: block.ID().Index()})
	}

	v.events.BlockTracked.Trigger(block)
}

// MarkerVotersTotalWeight retrieves Validators supporting a given marker.
func (v *VirtualVoting) MarkerVotersTotalWeight(marker markers.Marker) (totalWeight int64) {
	v.evictionMutex.RLock()
	defer v.evictionMutex.RUnlock()

	_ = v.sequenceTracker.Voters(marker).ForEach(func(id identity.ID) error {
		if weight, exists := v.Validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})

	return totalWeight
}

// SlotVotersTotalWeight retrieves the total weight of the Validators voting for a given slot.
func (v *VirtualVoting) SlotVotersTotalWeight(slotIndex slot.Index) (totalWeight int64) {
	v.evictionMutex.RLock()
	defer v.evictionMutex.RUnlock()

	_ = v.slotTracker.Voters(slotIndex).ForEach(func(id identity.ID) error {
		if weight, exists := v.Validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})

	return totalWeight
}

// ConflictVoters retrieves Validators voting for a given conflict.
func (v *VirtualVoting) ConflictVoters(conflictID utxo.TransactionID) (voters *sybilprotection.WeightedSet) {
	v.evictionMutex.RLock()
	defer v.evictionMutex.RUnlock()

	return v.Validators.Weights.NewWeightedSet(v.conflictTracker.Voters(conflictID).Slice()...)
}

// ConflictVotersTotalWeight retrieves the total weight of the Validators voting for a given conflict.
func (v *VirtualVoting) ConflictVotersTotalWeight(conflictID utxo.TransactionID) (totalWeight int64) {
	v.evictionMutex.RLock()
	defer v.evictionMutex.RUnlock()

	if conflict, exists := v.ConflictDAG.Conflict(conflictID); exists {
		if conflict.ConfirmationState().IsAccepted() {
			return math.MaxInt64
		} else if conflict.ConfirmationState().IsRejected() {
			return 0
		}
	}

	_ = v.conflictTracker.Voters(conflictID).ForEach(func(id identity.ID) error {
		if weight, exists := v.Validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})
	return totalWeight
}

func (v *VirtualVoting) EvictSequence(sequenceID markers.SequenceID) {
	v.evictionMutex.Lock()
	defer v.evictionMutex.Unlock()

	v.sequenceTracker.EvictSequence(sequenceID)
}

func (v *VirtualVoting) EvictSlotTracker(slotIndex slot.Index) {
	v.evictionMutex.Lock()
	defer v.evictionMutex.Unlock()

	v.slotTracker.EvictSlot(slotIndex)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Forking logic ////////////////////////////////////////////////////////////////////////////////////////////////

// ProcessForkedBlock updates the Conflict weight after an individually mapped Block was forked into a new Conflict.
func (v *VirtualVoting) ProcessForkedBlock(block *booker.Block, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	votePower := booker.NewBlockVotePower(block.ID(), block.IssuingTime())

	// Do not apply votes of subjectively invalid blocks on forking. Votes of subjectively invalid blocks are also not counted
	// when booking.
	if block.IsSubjectivelyInvalid() {
		return
	}

	v.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, block.IssuerID(), votePower)
}

func (v *VirtualVoting) ProcessForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	// take everything in future cone because it was not conflicting before and move to new conflict.
	for voterID, votePower := range v.sequenceTracker.VotersWithPower(marker) {
		v.conflictTracker.AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, voterID, votePower)
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
