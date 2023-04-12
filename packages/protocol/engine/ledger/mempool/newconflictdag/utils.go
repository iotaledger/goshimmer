package newconflictdag

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// heaviestConflict returns the largest Conflict from the given Conflicts.
func heaviestConflict[ConflictID, ResourceID IDType, VoterPower constraints.Comparable[VoterPower]](conflicts *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VoterPower]]) *Conflict[ConflictID, ResourceID, VoterPower] {
	var result *Conflict[ConflictID, ResourceID, VoterPower]
	_ = conflicts.ForEach(func(conflict *Conflict[ConflictID, ResourceID, VoterPower]) (err error) {
		if conflict.Compare(result) == weight.Heavier {
			result = conflict
		}

		return nil
	})

	return result
}
