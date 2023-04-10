package newconflictdag

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/walker"
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

// walkConflicts walks past the given Conflicts and calls the callback for each Conflict.
func walkConflicts[ConflictID, ResourceID conflict.IDType](callback func(currentConflict *conflict.Conflict[ConflictID, ResourceID], walk *walker.Walker[*conflict.Conflict[ConflictID, ResourceID]]) error, conflicts ...*conflict.Conflict[ConflictID, ResourceID]) error {
	for walk := walker.New[*conflict.Conflict[ConflictID, ResourceID]]().PushAll(conflicts...); walk.HasNext(); {
		if err := callback(walk.Next(), walk); err != nil {
			return xerrors.Errorf("failed to walk past cone: %w", err)
		}
	}

	return nil
}

func walkPastCone[ConflictID, ResourceID conflict.IDType](callback func(currentConflict *conflict.Conflict[ConflictID, ResourceID]) error, conflicts ...*conflict.Conflict[ConflictID, ResourceID]) error {
	return walkConflicts(func(currentConflict *conflict.Conflict[ConflictID, ResourceID], walk *walker.Walker[*conflict.Conflict[ConflictID, ResourceID]]) error {
		if err := callback(currentConflict); err != nil {
			return xerrors.Errorf("failed to walk future cone: %w", err)
		}

		walk.PushAll(currentConflict.Parents.Slice()...)

		return nil
	}, conflicts...)
}

func walkFutureCone[ConflictID, ResourceID conflict.IDType](callback func(currentConflict *conflict.Conflict[ConflictID, ResourceID]) error, conflicts ...*conflict.Conflict[ConflictID, ResourceID]) error {
	return walkConflicts(func(c *conflict.Conflict[ConflictID, ResourceID], walk *walker.Walker[*conflict.Conflict[ConflictID, ResourceID]]) error {
		if err := callback(c); err != nil {
			return xerrors.Errorf("failed to walk future cone: %w", err)
		}

		walk.PushAll(c.Children.Slice()...)

		return nil
	}, conflicts...)
}
