package booker

import (
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/module"
)

type Booker interface {
	Events() *Events

	VirtualVoting() VirtualVoting

	// Block retrieves a Block with metadata from the in-memory storage of the Booker.
	Block(id models.BlockID) (block *Block, exists bool)

	// BlockCeiling returns the smallest Index that is >= the given Marker and a boolean value indicating if it exists.
	BlockCeiling(marker markers.Marker) (ceilingMarker markers.Marker, exists bool)

	// BlockFromMarker retrieves the Block of the given Marker.
	BlockFromMarker(marker markers.Marker) (block *Block, exists bool)

	// BlockConflicts returns the Conflict related details of the given Block.
	BlockConflicts(block *Block) (blockConflictIDs utxo.TransactionIDs)

	// BlockBookingDetails returns the Conflict and Marker related details of the given Block.
	BlockBookingDetails(block *Block) (pastMarkersConflictIDs, blockConflictIDs utxo.TransactionIDs)

	// TransactionConflictIDs returns the ConflictIDs of the Transaction contained in the given Block including conflicts from the UTXO past cone.
	TransactionConflictIDs(block *Block) (conflictIDs utxo.TransactionIDs)

	// GetEarliestAttachment returns the earliest attachment for a given transaction ID.
	// returnOrphaned parameter specifies whether the returned attachment may be orphaned.
	GetEarliestAttachment(txID utxo.TransactionID) (attachment *Block)

	// GetLatestAttachment returns the latest attachment for a given transaction ID.
	// returnOrphaned parameter specifies whether the returned attachment may be orphaned.
	GetLatestAttachment(txID utxo.TransactionID) (attachment *Block)

	SequenceManager() *markers.SequenceManager

	SequenceTracker() *sequencetracker.SequenceTracker[BlockVotePower]

	// MarkerVotersTotalWeight retrieves Validators supporting a given marker.
	MarkerVotersTotalWeight(marker markers.Marker) (totalWeight int64)

	// SlotVotersTotalWeight retrieves the total weight of the Validators voting for a given slot.
	SlotVotersTotalWeight(slotIndex slot.Index) (totalWeight int64)

	GetAllAttachments(txID utxo.TransactionID) (attachments *advancedset.AdvancedSet[*Block])

	module.Interface
}

type VirtualVoting interface {
	Events() *VirtualVotingEvents

	ConflictTracker() *conflicttracker.ConflictTracker[utxo.TransactionID, utxo.OutputID, BlockVotePower]

	// ConflictVotersTotalWeight retrieves the total weight of the Validators voting for a given conflict.
	ConflictVotersTotalWeight(conflictID utxo.TransactionID) (totalWeight int64)

	// ConflictVoters retrieves Validators voting for a given conflict.
	ConflictVoters(conflictID utxo.TransactionID) (voters *sybilprotection.WeightedSet)
}
