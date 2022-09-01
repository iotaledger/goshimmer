package notarization

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/mana"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the existing events of a notarization manager.
type Events struct {
	// EpochCommittable is an event that gets triggered whenever an epoch commitment is committable.
	EpochCommittable *event.Event[*EpochCommittableEvent]
	// EpochConfirmed is an event that gets triggered whenever an epoch is confirmed.
	EpochConfirmed *event.Event[*EpochConfirmedEvent]
	// CompetingCommitmentDetected is an event that gets triggered whenever a competing epoch commitment is detected.
	CompetingCommitmentDetected *event.Event[*CompetingCommitmentDetectedEvent]
	// ManaVectorUpdate is an event that gets triggered whenever the consensus mana vector needs to be updated.
	ManaVectorUpdate *event.Event[*mana.ManaVectorUpdateEvent]
	// TangleTreeInserted is an event that gets triggered when a Block is inserted into the Tangle smt.
	TangleTreeInserted *event.Event[*TangleTreeUpdatedEvent]
	// TangleTreeRemoved is an event that gets triggered when a Block is removed from Tangle smt.
	TangleTreeRemoved *event.Event[*TangleTreeUpdatedEvent]
	// StateMutationTreeInserted is an event that gets triggered when a transaction is inserted into the state mutation smt.
	StateMutationTreeInserted *event.Event[*StateMutationTreeUpdatedEvent]
	// StateMutationTreeRemoved is an event that gets triggered when a transaction is removed from state mutation smt.
	StateMutationTreeRemoved *event.Event[*StateMutationTreeUpdatedEvent]
	// UTXOTreeInserted is an event that gets triggered when UTXOs are stored into the UTXO smt.
	UTXOTreeInserted *event.Event[*UTXOUpdatedEvent]
	// UTXOTreeRemoved is an event that gets triggered when UTXOs are removed from the UTXO smt.
	UTXOTreeRemoved *event.Event[*UTXOUpdatedEvent]
	// Bootstrapped is an event that gets triggered when a notarization manager has the last committable epoch relatively close to current epoch.
	Bootstrapped *event.Event[*BootstrappedEvent]
	// SyncRange is an event that gets triggered when an entire range of epochs needs to be requested, validated and solidified
	SyncRange *event.Event[*SyncRangeEvent]
	// ActivityTreeInserted is an event that gets triggered when nodeID is added to the activity tree.
	ActivityTreeInserted *event.Event[*ActivityTreeUpdatedEvent]
	// ActivityTreeRemoved is an event that gets triggered when nodeID is removed from activity tree.
	ActivityTreeRemoved *event.Event[*ActivityTreeUpdatedEvent]
}

// TangleTreeUpdatedEvent is a container that acts as a dictionary for the TangleTree inserted/removed event related parameters.
type TangleTreeUpdatedEvent struct {
	// EI is the index of the block.
	EI epoch.Index
	// BlockID is the blockID that inserted/removed to/from the tangle smt.
	BlockID tangleold.BlockID
}

// BootstrappedEvent is an event that gets triggered when a notarization manager has the last committable epoch relatively close to current epoch.
type BootstrappedEvent struct {
	// EI is the index of the last commitable epoch
	EI epoch.Index
}

// StateMutationTreeUpdatedEvent is a container that acts as a dictionary for the State mutation tree inserted/removed event related parameters.
type StateMutationTreeUpdatedEvent struct {
	// EI is the index of the transaction.
	EI epoch.Index
	// TransactionID is the transaction ID that inserted/removed to/from the state mutation smt.
	TransactionID utxo.TransactionID
}

// UTXOUpdatedEvent is a container that acts as a dictionary for the UTXO update event related parameters.
type UTXOUpdatedEvent struct {
	// EI is the index of updated UTXO.
	EI epoch.Index
	// Spent are outputs that is spent in a transaction.
	Spent []*ledger.OutputWithMetadata
	// Created are the outputs created in a transaction.
	Created []*ledger.OutputWithMetadata
}

// EpochCommittableEvent is a container that acts as a dictionary for the EpochCommittable event related parameters.
type EpochCommittableEvent struct {
	// EI is the index of committable epoch.
	EI epoch.Index
	// ECRecord is the ec root of committable epoch.
	ECRecord *epoch.ECRecord
}

// EpochConfirmedEvent is a container that acts as a dictionary for the EpochConfirmed event related parameters.
type EpochConfirmedEvent struct {
	// EI is the index of committable epoch.
	EI epoch.Index
}

// CompetingCommitmentDetectedEvent is a container that acts as a dictionary for the CompetingCommitmentDetectedEvent event related parameters.
type CompetingCommitmentDetectedEvent struct {
	// Block is the block that contains the competing commitment.
	Block *tangleold.Block
}

// SyncRangeEvent is a container that acts as a dictionary for the SyncRange event related parameters.
type SyncRangeEvent struct {
	StartEI   epoch.Index
	EndEI     epoch.Index
	StartEC   epoch.EC
	EndPrevEC epoch.EC
}

// ActivityTreeUpdatedEvent is a container that acts as a dictionary for the ActivityTree inserted/removed event related parameters.
type ActivityTreeUpdatedEvent struct {
	// EI is the index of the epoch.
	EI epoch.Index
	// NodeID is the issuer nodeID.
	NodeID identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
