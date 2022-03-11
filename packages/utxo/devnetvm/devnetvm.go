package devnetvm

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/utxo"
)

type DevNetVM struct {
}

func (d *DevNetVM) ExecuteTransaction(transaction utxo.Transaction, inputs []utxo.Output, executionLimit ...uint64) (outputs []utxo.Output, err error) {
	typedTransaction := transaction.(*Transaction)

	// TODO implement me
	panic(fmt.Sprintf("%s", typedTransaction))
}

var _ utxo.VM = new(DevNetVM)
