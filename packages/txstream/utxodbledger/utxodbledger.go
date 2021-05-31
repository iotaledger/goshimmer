package utxodbledger

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxodb"
)

// UtxoDBLedger implements txstream.Ledger by wrapping UTXODB
type UtxoDBLedger struct {
	*utxodb.UtxoDB
	txConfirmedEvent *events.Event
	txBookedEvent    *events.Event
	log              *logger.Logger
}

var txEventHandler = func(f interface{}, params ...interface{}) {
	f.(func(tx *ledgerstate.Transaction))(params[0].(*ledgerstate.Transaction))
}

// New creates a new empty ledger
func New(log *logger.Logger) *UtxoDBLedger {
	return &UtxoDBLedger{
		UtxoDB:           utxodb.New(),
		txConfirmedEvent: events.NewEvent(txEventHandler),
		txBookedEvent:    events.NewEvent(txEventHandler),
		log:              log.Named("txstream/UtxoDBLedger"),
	}
}

// PostTransaction posts a transaction to the ledger
func (u *UtxoDBLedger) PostTransaction(tx *ledgerstate.Transaction) error {
	_, ok := u.UtxoDB.GetTransaction(tx.ID())
	if ok {
		u.log.Debugf("PostTransaction: tx already in ledger: %s", tx.ID().Base58())
		return nil
	}
	err := u.AddTransaction(tx)
	if err == nil {
		u.txConfirmedEvent.Trigger(tx)
	}
	return err
}

// GetUnspentOutputs returns the available UTXOs for an address
func (u *UtxoDBLedger) GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output)) {
	for _, out := range u.GetAddressOutputs(addr) {
		f(out)
	}
}

// GetConfirmedTransaction fetches a transaction by ID, and executes the given callback if found
func (u *UtxoDBLedger) GetConfirmedTransaction(txid ledgerstate.TransactionID, f func(*ledgerstate.Transaction)) bool {
	tx, ok := u.UtxoDB.GetTransaction(txid)
	if ok {
		f(tx)
	}
	return ok
}

// GetTxInclusionState returns the inclusion state of the given transaction
func (u *UtxoDBLedger) GetTxInclusionState(txid ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	_, ok := u.UtxoDB.GetTransaction(txid)
	if !ok {
		return ledgerstate.Pending, xerrors.Errorf("UtxoDBLedger.GetTxInclusionState: not found %s", txid.Base58())
	}
	return ledgerstate.Confirmed, nil
}

// RequestFunds requests funds from the faucet
func (u *UtxoDBLedger) RequestFunds(target ledgerstate.Address) error {
	_, err := u.UtxoDB.RequestFunds(target)
	return err
}

// EventTransactionConfirmed returns an event that triggers when a transaction is confirmed
func (u *UtxoDBLedger) EventTransactionConfirmed() *events.Event {
	return u.txConfirmedEvent
}

// EventTransactionBooked returns an event that triggers when a transaction is booked
func (u *UtxoDBLedger) EventTransactionBooked() *events.Event {
	return u.txBookedEvent
}

// Detach detaches the event handlers
func (u *UtxoDBLedger) Detach() {}
