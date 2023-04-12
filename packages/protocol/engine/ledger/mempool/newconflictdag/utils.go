package newconflictdag

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// heaviestConflict returns the largest Conflict from the given Conflicts.
func heaviestConflict[ConflictID, ResourceID conflict.IDType, VoterPower constraints.Comparable[VoterPower]](conflicts *advancedset.AdvancedSet[*conflict.Conflict[ConflictID, ResourceID, VoterPower]]) *conflict.Conflict[ConflictID, ResourceID, VoterPower] {
	var result *conflict.Conflict[ConflictID, ResourceID, VoterPower]
	_ = conflicts.ForEach(func(conflict *conflict.Conflict[ConflictID, ResourceID, VoterPower]) (err error) {
		if conflict.Compare(result) == weight.Heavier {
			result = conflict
		}

		return nil
	})

	return result
}
