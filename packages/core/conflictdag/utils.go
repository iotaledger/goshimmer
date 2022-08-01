package conflictdag

import (
	"context"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/serix"
)

// Utils is a ConflictDAG component that bundles utility related API to simplify common interactions with the ConflictDAG.
type Utils[ConflictID comparable, ConflictSetID comparable] struct {
	// conflictDAG contains a reference to the ConflictDAG that created the Utils.
	conflictDAG *ConflictDAG[ConflictID, ConflictSetID]
}

// newUtils returns a new Utils instance for the given ConflictDAG.
func newUtils[ConflictID comparable, ConflictSetID comparable](conflictDAG *ConflictDAG[ConflictID, ConflictSetID]) (new *Utils[ConflictID, ConflictSetID]) {
	return &Utils[ConflictID, ConflictSetID]{
		conflictDAG: conflictDAG,
	}
}

func (u *Utils[ConflictID, ConflictSetID]) ForEachChildConflictID(conflictID ConflictID, callback func(childConflictID ConflictID)) {
	u.conflictDAG.Storage.CachedChildConflicts(conflictID).Consume(func(childConflict *ChildConflict[ConflictID]) {
		callback(childConflict.ChildConflictID())
	})
}

// ForEachConflict iterates over every existing Conflict in the entire Storage.
func (u *Utils[ConflictID, ConflictSetID]) ForEachConflict(consumer func(conflict *Conflict[ConflictID, ConflictSetID])) {
	u.conflictDAG.Storage.conflictStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Conflict[ConflictID, ConflictSetID]]) bool {
		cachedObject.Consume(func(conflict *Conflict[ConflictID, ConflictSetID]) {
			consumer(conflict)
		})

		return true
	})
}

// ForEachConflictingConflictID executes the callback for each Conflict that is conflicting with the named Conflict.
func (u *Utils[ConflictID, ConflictSetID]) ForEachConflictingConflictID(conflictID ConflictID, callback func(conflictingConflictID ConflictID) bool) {
	u.conflictDAG.Storage.CachedConflict(conflictID).Consume(func(conflict *Conflict[ConflictID, ConflictSetID]) {
		u.forEachConflictingConflictID(conflict, callback)
	})
}

// ForEachConnectedConflictingConflictID executes the callback for each Conflict that is directly or indirectly connected to
// the named Conflict through a chain of intersecting conflicts.
func (u *Utils[ConflictID, ConflictSetID]) ForEachConnectedConflictingConflictID(conflictID ConflictID, callback func(conflictingConflictID ConflictID)) {
	traversedConflicts := set.New[ConflictID]()
	conflictSetsWalker := walker.New[ConflictSetID]()

	processConflictAndQueueConflictSets := func(conflictID ConflictID) {
		if !traversedConflicts.Add(conflictID) {
			return
		}

		u.conflictDAG.Storage.CachedConflict(conflictID).Consume(func(conflict *Conflict[ConflictID, ConflictSetID]) {
			_ = conflict.ConflictSetIDs().ForEach(func(conflictSetID ConflictSetID) (err error) {
				conflictSetsWalker.Push(conflictSetID)
				return nil
			})
		})
	}

	processConflictAndQueueConflictSets(conflictID)

	for conflictSetsWalker.HasNext() {
		u.conflictDAG.Storage.CachedConflictMembers(conflictSetsWalker.Next()).Consume(func(conflictMember *ConflictMember[ConflictSetID, ConflictID]) {
			processConflictAndQueueConflictSets(conflictMember.ConflictID())
		})
	}

	traversedConflicts.ForEach(callback)
}

// forEachConflictingConflictID executes the callback for each Conflict that is conflicting with the named Conflict.
func (u *Utils[ConflictID, ConflictSetID]) forEachConflictingConflictID(conflict *Conflict[ConflictID, ConflictSetID], callback func(conflictingConflictID ConflictID) bool) {
	for it := conflict.ConflictSetIDs().Iterator(); it.HasNext(); {
		abort := false
		u.conflictDAG.Storage.CachedConflictMembers(it.Next()).Consume(func(conflictMember *ConflictMember[ConflictSetID, ConflictID]) {
			if abort || conflictMember.ConflictID() == conflict.ID() {
				return
			}

			if abort = !callback(conflictMember.ConflictID()); abort {
				it.StopWalk()
			}
		})
	}
}

// bytes is an internal utility function that simplifies the serialization of the identifier types.
func bytes(obj interface{}) (bytes []byte) {
	return lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), obj))
}
