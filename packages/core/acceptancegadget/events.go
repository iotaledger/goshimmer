package acceptancegadget

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type Events struct {
	BlockAccepted *event.Event[*Block]
	Reorg         *event.Event[utxo.TransactionID]

	Error *event.Event[error]
}

// newEvents creates a new Events instance.
func newEvents() *Events {
	return &Events{
		BlockAccepted: event.New[*Block](),
		Reorg:         event.New[utxo.TransactionID](),
		Error:         event.New[error](),
	}
}
