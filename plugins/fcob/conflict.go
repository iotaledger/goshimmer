package fcob

import (
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

type fcobConflict struct{}

func (fcobConflict) GetConflictSet(target ternary.Trinary) (conflictSet map[ternary.Trinary]bool) {
	txObject, err := tangle.GetTransaction(target)
	if err != nil {
		//TODO: handle error
	}

	targetAddress := txObject.GetAddress()
	conflictSet = make(map[ternary.Trinary]bool)
	conflict := false
	// In real implementation we don't need to iterate the all tangle
	// since we can just use the ledger state.
	for txHash, txObject := range dummyTangle {
		if targetAddress == txObject.address {
			if target != txHash { // filter out the same target tx
				conflictSet[txHash] = true
				conflict = true
			}
		}
	}
	if conflict {
		conflictSet[target] = true
	}
	return conflictSet
}
