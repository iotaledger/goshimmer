package tangleledger

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/txstream"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/events"
)

// TangleLedger imlpements txstream.TangleLedger with the GoShimmer tangle as backend
type TangleLedger struct {
	txConfirmedClosure *events.Closure
	txBookedClosure    *events.Closure

	txConfirmedEvent *events.Event
	txBookedEvent    *events.Event
}

// ensure conformance to Ledger interface
var _ txstream.Ledger = &TangleLedger{}

var txEventHandler = func(f interface{}, params ...interface{}) {
	f.(func(tx *ledgerstate.Transaction))(params[0].(*ledgerstate.Transaction))
}

type dependencies struct {
	dig.In

	Tangle *tangle.Tangle
}

var deps dependencies

// New returns an implementation for txstream.Ledger
func New() *TangleLedger {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		panic(err)
	}

	t := &TangleLedger{
		txConfirmedEvent: events.NewEvent(txEventHandler),
		txBookedEvent:    events.NewEvent(txEventHandler),
	}

	t.txConfirmedClosure = events.NewClosure(func(id ledgerstate.TransactionID) {
		deps.Tangle.LedgerState.UTXODAG.CachedTransaction(id).Consume(func(transaction *ledgerstate.Transaction) {
			go t.txConfirmedEvent.Trigger(transaction)
		})
	})
	deps.Tangle.LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(t.txConfirmedClosure)

	t.txBookedClosure = events.NewClosure(func(id tangle.MessageID) {
		deps.Tangle.Storage.Message(id).Consume(func(msg *tangle.Message) {
			if payload := msg.Payload(); payload != nil && payload.Type() == ledgerstate.TransactionType {
				go t.txBookedEvent.Trigger(payload)
			}
		})
	})
	deps.Tangle.Booker.Events.MessageBooked.Attach(t.txBookedClosure)

	return t
}

// Detach detaches the event handlers
func (t *TangleLedger) Detach() {
	deps.Tangle.LedgerState.UTXODAG.Events().TransactionConfirmed.Detach(t.txConfirmedClosure)
	deps.Tangle.Booker.Events.MessageBooked.Detach(t.txBookedClosure)
}

// EventTransactionConfirmed returns an event that triggers when a transaction is confirmed
func (t *TangleLedger) EventTransactionConfirmed() *events.Event {
	return t.txConfirmedEvent
}

// EventTransactionBooked returns an event that triggers when a transaction is booked
func (t *TangleLedger) EventTransactionBooked() *events.Event {
	return t.txBookedEvent
}

// GetUnspentOutputs returns the available UTXOs for an address
func (t *TangleLedger) GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output)) {
	deps.Tangle.LedgerState.CachedOutputsOnAddress(addr).Consume(func(output ledgerstate.Output) {
		ok := true
		if state, err := t.GetTxInclusionState(output.ID().TransactionID()); err != nil || state != ledgerstate.Confirmed {
			return
		}
		deps.Tangle.LedgerState.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
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

// GetConfirmedTransaction fetches a transaction by ID, and executes the given callback if found
func (t *TangleLedger) GetConfirmedTransaction(txid ledgerstate.TransactionID, f func(ret *ledgerstate.Transaction)) (found bool) {
	found = false
	deps.Tangle.LedgerState.Transaction(txid).Consume(func(tx *ledgerstate.Transaction) {
		state, _ := t.GetTxInclusionState(txid)
		if state == ledgerstate.Confirmed {
			found = true
			f(tx)
		}
	})
	return
}

// GetTxInclusionState returns the inclusion state of the given transaction
func (t *TangleLedger) GetTxInclusionState(txid ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	return deps.Tangle.LedgerState.TransactionInclusionState(txid)
}

// PostTransaction posts a transaction to the ledger
func (t *TangleLedger) PostTransaction(tx *ledgerstate.Transaction) error {
	_, err := deps.Tangle.IssuePayload(tx)
	if err != nil {
		return fmt.Errorf("failed to issue transaction: %w", err)
	}
	return nil
}

// GetOutput finds an output by ID (either spent or unspent)
func (t *TangleLedger) GetOutput(outID ledgerstate.OutputID, f func(ledgerstate.Output)) bool {
	return deps.Tangle.LedgerState.CachedOutput(outID).Consume(f)
}

// GetOutputMetadata finds an output by ID and returns its metadata
func (t *TangleLedger) GetOutputMetadata(outID ledgerstate.OutputID, f func(*ledgerstate.OutputMetadata)) bool {
	return deps.Tangle.LedgerState.CachedOutputMetadata(outID).Consume(f)
}
