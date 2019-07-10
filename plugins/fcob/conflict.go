package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/iota.go/trinary"
)

// getConflictSet triggers a (fake) new conflict every 10 received txs
// including only 1 conflicting tx in the returned conflict set
func getConflictSet(transaction trinary.Trytes, tangle tangleAPI) (conflictSet map[trinary.Trytes]bool, err errors.IdentifiableError) {

	conflictSet = make(map[trinary.Trytes]bool)

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
