package ledger

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/syncutils"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	TransactionStoredEvent *event.Event[TransactionID]
	TransactionBookedEvent *event.Event[TransactionID]
	ErrorEvent             *event.Event[error]

	*Storage
	*Solidifier
	*Validator
	*VM
	*Booker

	*DataFlow
	*Options
	*Utils

	*branchdag.BranchDAG
	*syncutils.DAGMutex[[32]byte]
}

func New(store kvstore.KVStore, vm utxo.VM, options ...Option) (ledger *Ledger) {
	ledger = &Ledger{
		TransactionStoredEvent: event.New[TransactionID](),
		TransactionBookedEvent: event.New[TransactionID](),
		ErrorEvent:             event.New[error](),

		BranchDAG: branchdag.NewBranchDAG(store, database.NewCacheTimeProvider(0)),
		DAGMutex:  syncutils.NewDAGMutex[[32]byte](),
	}

	ledger.Configure(options...)

	ledger.DataFlow = NewDataFlow(ledger)
	ledger.Storage = NewStorage(ledger)
	ledger.Solidifier = NewSolidifier(ledger)
	ledger.Validator = NewValidator(ledger)
	ledger.VM = NewVM(ledger, vm)
	ledger.Booker = NewBooker(ledger)
	ledger.Utils = NewUtils(ledger)

	return ledger
}

func (l *Ledger) Configure(options ...Option) {
	if l.Options == nil {
		l.Options = &Options{
			Store:              mapdb.NewMapDB(),
			CacheTimeProvider:  database.NewCacheTimeProvider(0),
			LazyBookingEnabled: true,
		}
	}

	for _, option := range options {
		option(l.Options)
	}
}

func (l *Ledger) StoreAndProcessTransaction(tx utxo.Transaction) (err error) {
	l.Lock(tx.ID())
	defer l.Unlock(tx.ID())

	return l.DataFlow.storeAndProcessTransaction().Run(&dataFlowParams{Transaction: NewTransaction(tx)})
}

func (l *Ledger) CheckTransaction(tx utxo.Transaction) (err error) {
	return l.DataFlow.checkTransaction().Run(&dataFlowParams{Transaction: NewTransaction(tx)})
}

func (l *Ledger) processTransaction(tx *Transaction) (err error) {
	l.Lock(tx.ID())
	defer l.Unlock(tx.ID())

	return l.DataFlow.processTransaction().Run(&dataFlowParams{Transaction: tx})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
