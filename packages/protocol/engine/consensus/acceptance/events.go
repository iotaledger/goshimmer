package acceptance

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Events struct {
	BlockAccepted  *event.Linkable[*Block]
	BlockConfirmed *event.Linkable[*Block]
	EpochClosed    *event.Linkable[*memstorage.Storage[models.BlockID, *Block]]
	Reorg          *event.Linkable[utxo.TransactionID]
	Error          *event.Linkable[error]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockAccepted: event.NewLinkable[*Block](),
		EpochClosed:   event.NewLinkable[*memstorage.Storage[models.BlockID, *Block]](),
		Reorg:         event.NewLinkable[utxo.TransactionID](),
		Error:         event.NewLinkable[error](),
	}
})
