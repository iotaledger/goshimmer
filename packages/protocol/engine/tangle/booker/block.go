package booker

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Block struct {
	booked                bool
	structureDetails      *markers.StructureDetails
	addedConflictIDs      utxo.TransactionIDs
	subtractedConflictIDs utxo.TransactionIDs

	*blockdag.Block
}

func (b *Block) Transaction() (tx utxo.Transaction, isTransaction bool) {
	tx, isTransaction = b.Payload().(utxo.Transaction)
	return tx, isTransaction
}

func (b *Block) AddAllAddedConflictIDs(addedConflictIDs utxo.TransactionIDs) {
	b.Lock()
	defer b.Unlock()

	if addedConflictIDs.IsEmpty() {
		return
	}

	b.addedConflictIDs.AddAll(addedConflictIDs)
}

// AddConflictID sets the ConflictIDs of the added Conflicts.
func (b *Block) AddConflictID(conflictID utxo.TransactionID) (modified bool) {
	b.Lock()
	defer b.Unlock()

	return b.addedConflictIDs.Add(conflictID)
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

func NewBlock(block *blockdag.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block:                 block,
		addedConflictIDs:      utxo.NewTransactionIDs(),
		subtractedConflictIDs: utxo.NewTransactionIDs(),
	}, opts)
}

func NewRootBlock(id models.BlockID, opts ...options.Option[models.Block]) *Block {
	blockDAGBlock := blockdag.NewRootBlock(id, opts...)

	genesisStructureDetails := markers.NewStructureDetails()
	genesisStructureDetails.SetIsPastMarker(true)
	genesisStructureDetails.SetPastMarkers(markers.NewMarkers(markers.NewMarker(0, 0)))

	return NewBlock(blockDAGBlock, WithBooked(true), WithStructureDetails(genesisStructureDetails))
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

// region Blocks ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Blocks represents a collection of Block.
type Blocks = *set.AdvancedSet[*Block]

// NewBlocks returns a new Block collection with the given elements.
func NewBlocks(blocks ...*Block) (newBlocks Blocks) {
	return set.NewAdvancedSet(blocks...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithBooked is a constructor Option for Blocks that initializes the given block with a specific missing flag.
func WithBooked(missing bool) options.Option[Block] {
	return func(block *Block) {
		block.booked = missing
	}
}

// WithStructureDetails is a constructor Option for Blocks that initializes the given block with a specific structure details.
func WithStructureDetails(structureDetails *markers.StructureDetails) options.Option[Block] {
	return func(block *Block) {
		block.structureDetails = structureDetails
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
