package tangleledger

import (
	"fmt"

	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"
)

// TangleLedger imlpements txstream.TangleLedger with the GoShimmer tangle as backend.
type TangleLedger struct {
	tangleInstance  *tangle.Tangle
	txBookedClosure *events.Closure
	txBookedEvent   *events.Event
}

// ensure conformance to Ledger interface.
var _ txstream.Ledger = &TangleLedger{}

var txEventHandler = func(f interface{}, params ...interface{}) {
	f.(func(tx *ledgerstate.Transaction))(params[0].(*ledgerstate.Transaction))
}

// New returns an implementation for txstream.Ledger.
func New(tangleInstance *tangle.Tangle) *TangleLedger {
	t := &TangleLedger{
		tangleInstance: tangleInstance,
		txBookedEvent:  events.NewEvent(txEventHandler),
	}

	t.txBookedClosure = events.NewClosure(func(id tangle.MessageID) {
		t.tangleInstance.Storage.Message(id).Consume(func(msg *tangle.Message) {
			if payload := msg.Payload(); payload != nil && payload.Type() == ledgerstate.TransactionType {
				go t.txBookedEvent.Trigger(payload)
			}
		})
	})
	t.tangleInstance.Booker.Events.MessageBooked.Attach(t.txBookedClosure)

	return t
}

// Detach detaches the event handlers.
func (t *TangleLedger) Detach() {
	t.tangleInstance.Booker.Events.MessageBooked.Detach(t.txBookedClosure)
}

// EventTransactionBooked returns an event that triggers when a transaction is booked.
func (t *TangleLedger) EventTransactionBooked() *events.Event {
	return t.txBookedEvent
}

// GetUnspentOutputs returns the available UTXOs for an address.
func (t *TangleLedger) GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output)) {
	t.tangleInstance.LedgerState.CachedOutputsOnAddress(addr).Consume(func(output ledgerstate.Output) {
		ok := true
		t.tangleInstance.LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			if outputMetadata.ConsumerCount() != 0 {
				ok = false
				return
			}
		})
		if ok {
			f(output)
		}
	})
}

// GetHighGoFTransaction fetches a transaction by ID, and executes the given callback if its GoF is high.
func (t *TangleLedger) GetHighGoFTransaction(txid ledgerstate.TransactionID, f func(ret *ledgerstate.Transaction)) (found bool) {
	if !t.tangleInstance.ConfirmationOracle.IsTransactionConfirmed(txid) {
		return false
	}

	t.tangleInstance.LedgerState.Transaction(txid).Consume(f)
	return true
}

// PostTransaction posts a transaction to the ledger.
func (t *TangleLedger) PostTransaction(tx *ledgerstate.Transaction) error {
	_, err := t.tangleInstance.IssuePayload(tx)
	if err != nil {
		return fmt.Errorf("failed to issue transaction: %w", err)
	}
	return nil
}

// GetOutput finds an output by ID (either spent or unspent).
func (t *TangleLedger) GetOutput(outID ledgerstate.OutputID, f func(ledgerstate.Output)) bool {
	return t.tangleInstance.LedgerState.CachedOutput(outID).Consume(f)
}

// GetOutputMetadata finds an output by ID and returns its metadata.
func (t *TangleLedger) GetOutputMetadata(outID ledgerstate.OutputID, f func(*ledgerstate.OutputMetadata)) bool {
	return t.tangleInstance.LedgerState.CachedOutputMetadata(outID).Consume(f)
}
