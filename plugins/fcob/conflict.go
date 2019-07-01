package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// getConflictSet triggers a (fake) new conflict every 10 received txs
// including only 1 conflicting tx in the returned conflict set
func getConflictSet(transaction ternary.Trinary, tangle tangleAPI) (conflictSet map[ternary.Trinary]bool, err errors.IdentifiableError) {

	conflictSet = make(map[ternary.Trinary]bool)

	txObject, err := tangle.GetTransaction(transaction)
	if err != nil {
		return conflictSet, err
	}
	conflict := txObject.GetValue()%10 == 0 // trigger a new conflict every 10 received txs
	if conflict {
		conflictSet[transaction] = true
	}
	return conflictSet, nil
}
