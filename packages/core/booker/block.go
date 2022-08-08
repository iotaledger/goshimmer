package booker

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

type Block struct {
	booked                bool
	structureDetails      *markers.StructureDetails
	addedConflictIDs      utxo.TransactionIDs
	subtractedConflictIDs utxo.TransactionIDs

	*tangle.Block
}

func (b *Block) AddAllAddedConflictIDs(addedConflictIDs utxo.TransactionIDs) {
	b.Lock()
	defer b.Unlock()

	if b.addedConflictIDs.IsEmpty() {
		return
	}

	addedConflictIDs.AddAll(addedConflictIDs)
}

// AddConflictID sets the ConflictIDs of the added Conflicts.
func (b *Block) AddConflictID(conflictID utxo.TransactionID) (modified bool) {
	b.Lock()
	defer b.Unlock()

	if b.addedConflictIDs.Has(conflictID) {
		return false
	}

	b.addedConflictIDs.Add(conflictID)
	return true
}

// AddedConflictIDs returns the ConflictIDs of the added Conflicts of the Block.
func (b *Block) AddedConflictIDs() utxo.TransactionIDs {
	b.RLock()
	defer b.RUnlock()

	return b.addedConflictIDs.Clone()
}

func (b *Block) AddAllSubtractedConflictIDs(subtractedConflictIDs utxo.TransactionIDs) {
	b.Lock()
	defer b.Unlock()

	if subtractedConflictIDs.IsEmpty() {
		return
	}

	b.subtractedConflictIDs.AddAll(subtractedConflictIDs)
}

// SubtractedConflictIDs returns the ConflictIDs of the subtracted Conflicts of the Block.
func (b *Block) SubtractedConflictIDs() utxo.TransactionIDs {
	b.RLock()
	defer b.RUnlock()

	return b.subtractedConflictIDs.Clone()
}

func NewBlock(block *tangle.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block:                 block,
		addedConflictIDs:      utxo.NewTransactionIDs(),
		subtractedConflictIDs: utxo.NewTransactionIDs(),
	}, opts)
}

func (b *Block) IsBooked() (isBooked bool) {
	b.RLock()
	defer b.RUnlock()

	return b.booked
}

func (b *Block) setBooked() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.booked; wasUpdated {
		b.booked = true
	}

	return
}

func (b *Block) StructureDetails() *markers.StructureDetails {
	b.RLock()
	defer b.RUnlock()

	return b.structureDetails
}

func (b *Block) setStructureDetails(structureDetails *markers.StructureDetails) {
	b.Lock()
	defer b.Unlock()

	b.structureDetails = structureDetails
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithBooked is a constructor Option for Blocks that initializes the given block with a specific missing flag.
func WithBooked(missing bool) options.Option[Block] {
	return func(block *Block) {
		block.booked = missing
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
