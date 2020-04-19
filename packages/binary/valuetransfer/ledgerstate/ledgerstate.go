package ledgerstate

import (
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/utxodag"
)

type LedgerState struct {
	utxoDAG *utxodag.UTXODAG
}

func New(utxoDAG *utxodag.UTXODAG) *LedgerState {
	return &LedgerState{
		utxoDAG: utxoDAG,
	}
}
