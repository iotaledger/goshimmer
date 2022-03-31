package ledger

import (
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type TransactionBookedEvent struct {
	TransactionID utxo.TransactionID
	Outputs       Outputs
}
