package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

type dummyConflict struct{}

// GetConflictSet triggers a (fake) new conflict every 10 received txs
// including only 1 conflicting tx in the returned conflict set
func (dummyConflict) GetConflictSet(target ternary.Trinary) (conflictSet map[ternary.Trinary]bool) {

	conflictSet = make(map[ternary.Trinary]bool)

	txObject, err := tangle.GetTransaction(target)
	if err != nil {
		//TODO: handle error
		PLUGIN.LogFailure(fmt.Sprintf("txObject: %v", txObject))
	}
	conflict := txObject.GetValue()%10 == 0 // trigger a new conflict every 10 received txs
	if conflict {
		PLUGIN.LogInfo(fmt.Sprintf("(GetConflictSet) NewConflict: %v", target))
		conflictSet[target] = true
	}
	return conflictSet
}
