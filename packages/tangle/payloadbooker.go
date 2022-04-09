package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/ledger"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
)

type PayloadBooker struct {
	Events *PayloadBookerEvents
	tangle *Tangle
}

func NewPayloadBooker(tangle *Tangle) *PayloadBooker {
	return &PayloadBooker{
		Events: &PayloadBookerEvents{
			PayloadBooked: events.NewEvent(MessageIDCaller),
		},
		tangle: tangle,
	}
}

func (p *PayloadBooker) Setup() {
	p.tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(p.bookPayload))
	p.tangle.Ledger.Events.TransactionBooked.Attach(event.NewClosure[*ledger.TransactionBookedEvent](func(event *ledger.TransactionBookedEvent) {
		p.processBookedTransaction(event.TransactionID)
	}))
}

// bookPayload books the Payload of a Message and returns its assigned BranchID.
func (p *PayloadBooker) bookPayload(messageID MessageID) {
	p.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		payload := message.Payload()
		if payload == nil || payload.Type() != devnetvm.TransactionType {
			p.Events.PayloadBooked.Trigger(messageID)
			return
		}

		tx := payload.(utxo.Transaction)

		err := p.tangle.Ledger.StoreAndProcessTransaction(tx)
		if err != nil {
			// TODO: handle invalid transactions
			fmt.Println(err)
		}
	})
	// TODO: store attachment -> depending on when we store attachment current message will be picked up automatically
	//  with processBookedTransaction
}

func (p *PayloadBooker) processBookedTransaction(id utxo.TransactionID) {
	p.tangle.Storage.Attachments(id).Consume(func(attachment *Attachment) {
		p.Events.PayloadBooked.Trigger(attachment.MessageID())
	})
}

// region PayloadBookerEvents //////////////////////////////////////////////////////////////////////////////////////////

// PayloadBookerEvents represents events happening in the PayloadBooker.
type PayloadBookerEvents struct {
	// PayloadBooked is triggered when a payload is booked. Data payloads are immediately considered as booked.
	PayloadBooked *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
