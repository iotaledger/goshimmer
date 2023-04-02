package markervirtualvoting

import (
	"fmt"
	"math"

	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region VirtualVoting ////////////////////////////////////////////////////////////////////////////////////////////////

type VirtualVoting struct {
	events          *booker.VirtualVotingEvents
	Validators      *sybilprotection.WeightedSet
	ConflictDAG     *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]
	sequenceManager *markers.SequenceManager

	conflictTracker *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, booker.BlockVotePower]

	Workers *workerpool.Group
}

func New(workers *workerpool.Group, conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], sequenceManager *markers.SequenceManager, validators *sybilprotection.WeightedSet) (newVirtualVoting *VirtualVoting) {
	newVirtualVoting = &VirtualVoting{
		events:          booker.NewVirtualVotingEvents(),
		Validators:      validators,
		Workers:         workers,
		ConflictDAG:     conflictDAG,
		sequenceManager: sequenceManager,
	}

	newVirtualVoting.conflictTracker = conflicttracker.NewConflictTracker[utxo.TransactionID, utxo.OutputID, booker.BlockVotePower](conflictDAG, validators)

	newVirtualVoting.events.ConflictTracker.LinkTo(newVirtualVoting.conflictTracker.Events)

	return
}

var _ booker.VirtualVoting = new(VirtualVoting)

func (v *VirtualVoting) Events() *booker.VirtualVotingEvents {
	return v.events
}

func (v *VirtualVoting) ConflictTracker() *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, booker.BlockVotePower] {
	return v.conflictTracker
}

func (v *VirtualVoting) Track(block *booker.Block, conflictIDs utxo.TransactionIDs, votePower booker.BlockVotePower) (invalid bool) {
	if _, invalid = v.conflictTracker.TrackVote(conflictIDs, block.IssuerID(), votePower); invalid {
		fmt.Println("block is subjectively invalid", block.ID())
		block.SetSubjectivelyInvalid(true)

		return true
	}

	return false
}

// ConflictVoters retrieves Validators voting for a given conflict.
func (v *VirtualVoting) ConflictVoters(conflictID utxo.TransactionID) (voters *sybilprotection.WeightedSet) {
	return v.Validators.Weights.NewWeightedSet(v.conflictTracker.Voters(conflictID).Slice()...)
}

// ConflictVotersTotalWeight retrieves the total weight of the Validators voting for a given conflict.
func (v *VirtualVoting) ConflictVotersTotalWeight(conflictID utxo.TransactionID) (totalWeight int64) {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
