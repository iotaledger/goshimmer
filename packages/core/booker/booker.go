package booker

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/marker"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

type Booker struct {
	// Events contains the Events of Tangle.
	Events *Events

	ledger        *ledger.Ledger
	bookingOrder  *causalorder.CausalOrder[models.BlockID, *Block]
	attachments   *attachments
	blocks        *memstorage.EpochStorage[models.BlockID, *Block]
	markerManager *MarkerManager
	bookingMutex  *syncutils.DAGMutex[models.BlockID]

	// rootBlockProvider contains a function that is used to retrieve the root Blocks of the Tangle.
	rootBlockProvider func(models.BlockID) *Block

	// TODO: finish shutdown implementation, maybe replace with component state
	// maxDroppedEpoch contains the highest epoch.Index that has been dropped from the Tangle.
	maxDroppedEpoch epoch.Index

	// TODO: finish shutdown implementation, maybe replace with component state
	// isShutdown contains a flag that indicates whether the Booker was shut down.
	isShutdown bool

	*tangle.Tangle
}

func New(tangleInstance *tangle.Tangle, ledgerInstance *ledger.Ledger, rootBlockProvider func(models.BlockID) *Block, opts ...options.Option[Booker]) (booker *Booker) {
	booker = options.Apply(&Booker{
		ledger:        ledgerInstance,
		Tangle:        tangleInstance,
		Events:        newEvents(),
		attachments:   newAttachments(),
		blocks:        memstorage.NewEpochStorage[models.BlockID, *Block](),
		markerManager: NewMarkerManager(),
		bookingMutex:  syncutils.NewDAGMutex[models.BlockID](),

		rootBlockProvider: rootBlockProvider,
	}, opts)
	booker.bookingOrder = causalorder.New(booker.Block, (*Block).IsBooked, (*Block).setBooked, causalorder.WithReferenceValidator[models.BlockID](isReferenceValid))
	booker.bookingOrder.Events.Emit.Hook(event.NewClosure(booker.book))
	booker.bookingOrder.Events.Drop.Attach(event.NewClosure(func(block *Block) { booker.SetInvalid(block.Block) }))

	tangleInstance.Events.BlockSolid.Attach(event.NewClosure(func(block *tangle.Block) {
		if _, err := booker.Queue(NewBlock(block)); err != nil {
			panic(err)
		}
	}))

	booker.ledger.Events.TransactionBooked.Attach(event.NewClosure(func(e *ledger.TransactionBookedEvent) {
		contextBlockID := models.BlockIDFromContext(e.Context)

		for _, block := range booker.attachments.Get(e.TransactionID) {
			if contextBlockID != block.ID() {
				booker.bookingOrder.Queue(block)
			}
		}
	}))

	return booker
}

func (b *Booker) Queue(block *Block) (wasQueued bool, err error) {
	if wasQueued, err = b.isPayloadSolid(block); wasQueued {
		fmt.Println("Payload solid, queuing")
		b.bookingOrder.Queue(block)
	}

	return
}

// Block retrieves a Block with metadata from the in-memory storage of the Tangle.
func (b *Booker) Block(id models.BlockID) (block *Block, exists bool) {
	b.RLock()
	defer b.RUnlock()

	if b.isShutdown {
		return nil, false
	}

	return b.block(id)
}

func (b *Booker) isPayloadSolid(block *Block) (isPayloadSolid bool, err error) {
	tx, isTx := block.Payload().(utxo.Transaction)
	if !isTx {
		return true, nil
	}

	b.attachments.Store(tx.ID(), block)

	if err = b.ledger.StoreAndProcessTransaction(
		models.BlockIDToContext(context.Background(), block.ID()), tx,
	); errors.Is(err, ledger.ErrTransactionUnsolid) {
		fmt.Println("Payload not solid")
		return false, nil
	}

	return err == nil, err
}

func (b *Booker) book(block *Block) {
	b.bookingMutex.RLock(block.Parents()...)
	defer b.bookingMutex.RUnlock(block.Parents()...)
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	fmt.Printf("booking %s, %p \n", block.ID(), block)

	// TODO: RLock parents to avoid race conditions
	//  Lock Block itself

	// collect all strong parents structure details
	parentsStructureDetails := make([]*marker.StructureDetails, 0)
	block.ForEachParentByType(models.StrongParentType, func(parentBlockID models.BlockID) bool {
		fmt.Println("parentBlockID", parentBlockID)
		parentBlock, exists := b.Block(parentBlockID)
		if !exists {
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}
		fmt.Printf("parentblock %s, %p\n", parentBlock.ID(), parentBlock)
		if parentBlock.StructureDetails() != nil {
			parentsStructureDetails = append(parentsStructureDetails, parentBlock.StructureDetails())
		}
		return true
	})

	// TODO: create MarkersManager component
	//  - mapping from Marker to Block
	//  - thresholdmap for Marker to conflicts mapping
	//  - abstract away all marker related stuff
	//  - manages pruning of markers and all related (conflict mapping) entities

	fmt.Println("parentsStructureDetails", parentsStructureDetails)
	newStructureDetails, newSequenceCreated := b.markerManager.ProcessBlock(block, parentsStructureDetails)
	block.setStructureDetails(newStructureDetails)
	fmt.Println(newSequenceCreated, newStructureDetails)
}

// block retrieves the Block with given id from the mem-storage.
func (b *Booker) block(id models.BlockID) (block *Block, exists bool) {
	if block = b.rootBlockProvider(id); block != nil {
		return block, true
	}

	if id.EpochIndex <= b.maxDroppedEpoch {
		return nil, false
	}

	return b.blocks.Get(id.EpochIndex, true).Get(id)
}

// isReferenceValid checks if the reference between the child and its parent is valid.
func isReferenceValid(child *Block, parent *Block) (isValid bool) {
	return !parent.IsInvalid()
}
