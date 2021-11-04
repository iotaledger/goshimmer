package utxodbledger

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxodb"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// UtxoDBLedger implements txstream.Ledger by wrapping UTXODB.
type UtxoDBLedger struct {
	*utxodb.UtxoDB
	tangleInstance   *tangle.Tangle
	txConfirmedEvent *events.Event
	txBookedEvent    *events.Event
	log              *logger.Logger
}

var txEventHandler = func(f interface{}, params ...interface{}) {
	f.(func(tx *ledgerstate.Transaction))(params[0].(*ledgerstate.Transaction))
}

// New creates a new empty ledger.
func New(log *logger.Logger, tangleInstance *tangle.Tangle) *UtxoDBLedger {
	return &UtxoDBLedger{
		UtxoDB:           utxodb.New(),
		tangleInstance:   tangleInstance,
		txConfirmedEvent: events.NewEvent(txEventHandler),
		txBookedEvent:    events.NewEvent(txEventHandler),
		log:              log.Named("txstream/UtxoDBLedger"),
	}
}

// PostTransaction posts a transaction to the ledger.
func (u *UtxoDBLedger) PostTransaction(tx *ledgerstate.Transaction) error {
	_, ok := u.UtxoDB.GetTransaction(tx.ID())
	if ok {
		u.log.Debugf("PostTransaction: tx already in ledger: %s", tx.ID().Base58())
		return nil
	}
	err := u.AddTransaction(tx)
	if err == nil {
		go u.txConfirmedEvent.Trigger(tx)
	}
	return err
}

// GetUnspentOutputs returns the available UTXOs for an address.
func (u *UtxoDBLedger) GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output)) {
	for _, out := range u.GetAddressOutputs(addr) {
		f(out)
	}
}

// GetHighGoFTransaction fetches a transaction by ID, and executes the given callback if its GoF is high.
func (u *UtxoDBLedger) GetHighGoFTransaction(txid ledgerstate.TransactionID, f func(ret *ledgerstate.Transaction)) (found bool) {
	found = false
	u.tangleInstance.LedgerState.TransactionMetadata(txid).Consume(func(txmeta *ledgerstate.TransactionMetadata) {
		if txmeta.GradeOfFinality() == gof.High {
			found = true
			u.tangleInstance.LedgerState.Transaction(txid).Consume(f)
		}
	})
	return
}

// RequestFunds requests funds from the faucet.
func (u *UtxoDBLedger) RequestFunds(target ledgerstate.Address) error {
	_, err := u.UtxoDB.RequestFunds(target)
	return err
}

// EventTransactionConfirmed returns an event that triggers when a transaction is confirmed.
func (u *UtxoDBLedger) EventTransactionConfirmed() *events.Event {
	return u.txConfirmedEvent
}

// EventTransactionBooked returns an event that triggers when a transaction is booked.
func (u *UtxoDBLedger) EventTransactionBooked() *events.Event {
	return u.txBookedEvent
}

// Detach detaches the event handlers.
func (u *UtxoDBLedger) Detach() {}
