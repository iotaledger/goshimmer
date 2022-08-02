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
	ledger      *ledger.Ledger
	orderer     *causalorder.CausalOrder[models.BlockID, *Block]
	attachments *Attachments

	*tangle.Tangle
}

func New(opts ...options.Option[Booker]) (newBooker *Booker) {
	return options.Apply(&Booker{}, opts).initOrder()
}

func (b *Booker) Queue(block *Block) (wasQueued bool, err error) {
	if wasQueued, err = b.isPayloadSolid(block); wasQueued {
		b.orderer.Queue(block)
	}

	return
}

func (b *Booker) isPayloadSolid(block *Block) (isPayloadSolid bool, err error) {
	tx, isTx := block.Payload().(utxo.Transaction)
	if !isTx {
		return true, nil
	}

	b.attachments.Store(tx, block)

	if err = b.ledger.StoreAndProcessTransaction(
		context.WithValue(context.Background(), "blockID", block.ID()), tx,
	); errors.Is(err, ledger.ErrTransactionUnsolid) {
		return false, nil
	}

	return err == nil, err
}

func (b *Booker) Block(id models.BlockID) (block *Block, exists bool) {
	return
}

// initSolidifier is used to lazily initialize the solidifier after the options have been populated.
func (b *Booker) initOrder(opts ...options.Option[causalorder.CausalOrder[models.BlockID, *Block]]) (self *Booker) {
	b.orderer = causalorder.New(b.Block, (*Block).IsBooked, (*Block).setBooked, opts...)
	b.orderer.Events.Emit.Hook(event.NewClosure(b.book))
	b.orderer.Events.Drop.Attach(event.NewClosure(func(block *Block) { b.SetInvalid(block.Block) }))

	return b
}

func (b *Booker) book(block *Block) {

}
