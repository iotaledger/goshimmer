package ledger

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Ledger struct {
	*Events
	utxo.VM
	*Storage
	*AvailabilityManager
}

func New(store kvstore.KVStore, vm utxo.VM) (ledger *Ledger) {
	ledger = new(Ledger)
	ledger.Events = newEvents()
	ledger.VM = vm
	ledger.Storage = NewStorage(ledger)
	ledger.AvailabilityManager = NewAvailabilityManager(ledger)

	return ledger
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Events struct {
	Error *events.Event
}

func newEvents() *Events {
	return &Events{
		Error: events.NewEvent(events.ErrorCaller),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
