package booker

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

type Booker struct {
	ledger       *ledger.Ledger
	bookingOrder *causalorder.CausalOrder[models.BlockID, *Block]
	attachments  *attachments

	*tangle.Tangle
}

func New(ledger *ledger.Ledger, opts ...options.Option[Booker]) (newBooker *Booker) {
	newBooker = options.Apply(&Booker{ledger: ledger}, opts)
	newBooker.bookingOrder = causalorder.New(newBooker.Block, (*Block).IsBooked, (*Block).setBooked, causalorder.WithReferenceValidator[models.BlockID](isReferenceValid))
	newBooker.bookingOrder.Events.Emit.Hook(event.NewClosure(newBooker.book))
	newBooker.bookingOrder.Events.Drop.Attach(event.NewClosure(func(block *Block) { newBooker.SetInvalid(block.Block) }))

	return newBooker
}

func (b *Booker) Queue(block *Block) (wasQueued bool, err error) {
	if wasQueued, err = b.isPayloadSolid(block); wasQueued {
		b.bookingOrder.Queue(block)
	}

	return
}

func (b *Booker) Block(id models.BlockID) (block *Block, exists bool) {
	return
}

// initSolidifier is used to lazily initialize the solidifier after the options have been populated.
func (b *Booker) init(opts ...options.Option[causalorder.CausalOrder[models.BlockID, *Block]]) (self *Booker) {

	return b
}

func (b *Booker) isPayloadSolid(block *Block) (isPayloadSolid bool, err error) {
	tx, isTx := block.Payload().(utxo.Transaction)
	if !isTx {
		return true, nil
	}

	b.attachments.Store(tx.ID(), block)

	if err = b.ledger.StoreAndProcessTransaction(
		context.WithValue(context.Background(), "blockID", block.ID()), tx,
	); errors.Is(err, ledger.ErrTransactionUnsolid) {
		return false, nil
	}

	return err == nil, err
}

func (b *Booker) book(block *Block) {

}

// isReferenceValid checks if the reference between the child and its parent is valid.
func isReferenceValid(child *Block, parent *Block) (isValid bool) {
	return !parent.IsInvalid()
}
