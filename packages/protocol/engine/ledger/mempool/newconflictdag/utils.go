package newconflictdag

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// largestConflict returns the largest Conflict from the given Conflicts.
func largestConflict[ConflictID, ResourceID conflict.IDType](conflicts *advancedset.AdvancedSet[*conflict.Conflict[ConflictID, ResourceID]]) *conflict.Conflict[ConflictID, ResourceID] {
	var largestConflict *conflict.Conflict[ConflictID, ResourceID]
	_ = conflicts.ForEach(func(conflict *conflict.Conflict[ConflictID, ResourceID]) (err error) {
		if conflict.Compare(largestConflict) == weight.Heavier {
			largestConflict = conflict
		}

		return nil
	})

	return largestConflict
}
