package booker

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/stringify"
)

type Block struct {
	booked                bool
	subjectivelyInvalid   bool
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

func (b *Block) IsSubjectivelyInvalid() bool {
	b.RLock()
	defer b.RUnlock()

	return b.subjectivelyInvalid
}

func (b *Block) SetSubjectivelyInvalid(bool) (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.subjectivelyInvalid; wasUpdated {
		b.subjectivelyInvalid = true
	}

	return
}

func NewBlock(block *blockdag.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		Block:                 block,
		addedConflictIDs:      utxo.NewTransactionIDs(),
		subtractedConflictIDs: utxo.NewTransactionIDs(),
	}, opts)
}

func NewRootBlock(id models.BlockID, slotTimeProvider *slot.TimeProvider, opts ...options.Option[models.Block]) *Block {
	blockDAGBlock := blockdag.NewRootBlock(id, slotTimeProvider, opts...)

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

func (b *Block) SetBooked() (wasUpdated bool) {
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

func (b *Block) SetStructureDetails(structureDetails *markers.StructureDetails) {
	b.Lock()
	defer b.Unlock()

	b.structureDetails = structureDetails
}

func (b *Block) String() string {
	builder := stringify.NewStructBuilder("VirtualVoting.Block", stringify.NewStructField("id", b.ID()))
	builder.AddField(stringify.NewStructField("Booked", b.booked))
	builder.AddField(stringify.NewStructField("SubjectivelyInvalid", b.subjectivelyInvalid))
	builder.AddField(stringify.NewStructField("StructureDetails", b.structureDetails))
	builder.AddField(stringify.NewStructField("AddedConflictIDs", b.addedConflictIDs))
	builder.AddField(stringify.NewStructField("SubtractedConflictIDs", b.subtractedConflictIDs))

	return builder.String()
}

// region Blocks ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Blocks represents a collection of Block.
type Blocks = *advancedset.AdvancedSet[*Block]

// NewBlocks returns a new Block collection with the given elements.
func NewBlocks(blocks ...*Block) (newBlocks Blocks) {
	return advancedset.New(blocks...)
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
