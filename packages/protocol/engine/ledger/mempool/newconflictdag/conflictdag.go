package newconflictdag

//
// import (
// 	"github.com/iotaledger/hive.go/ds/advancedset"
// 	"github.com/iotaledger/hive.go/ds/shrinkingmap"
// 	"github.com/iotaledger/hive.go/runtime/options"
// 	"github.com/iotaledger/hive.go/runtime/syncutils"
// )
//
// type ConflictDAG[ConflictIDType, ResourceIDType comparable] struct {
// 	conflicts    *shrinkingmap.ShrinkingMap[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]]
// 	conflictSets *shrinkingmap.ShrinkingMap[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]]
//
// 	mutex *syncutils.StarvingMutex
//
// 	optsMergeToMaster bool
// }
//
// func New[ConflictIDType comparable, ResourceIDType comparable](opts ...options.Option[ConflictDAG[ConflictIDType, ResourceIDType]]) *ConflictDAG[ConflictIDType, ResourceIDType] {
// 	return options.Apply(&ConflictDAG[ConflictIDType, ResourceIDType]{
// 		conflicts:    shrinkingmap.New[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]](),
// 		conflictSets: shrinkingmap.New[ResourceIDType, *ConflictSet[ConflictIDType, ResourceIDType]](),
// 		mutex:        syncutils.NewStarvingMutex(),
// 	}, opts, func(instance *ConflictDAG[ConflictIDType, ResourceIDType]) {
//
// 	})
// }
//
// func (c *ConflictDAG[ConflictIDType, ResourceIDType]) CreateConflict(id ConflictIDType, parentIDs *advancedset.AdvancedSet[ConflictIDType], conflictingResourceIDs *advancedset.AdvancedSet[ResourceIDType], initialWeight Weight) (created bool) {
// 	c.mutex.Lock()
// 	defer c.mutex.Unlock()
//
// 	conflictParents := advancedset.New[*Conflict[ConflictIDType, ResourceIDType]]()
// 	for it := parentIDs.Iterator(); it.HasNext(); {
// 		parentID := it.Next()
// 		parent, exists := c.conflicts.Get(parentID)
// 		if !exists {
// 			// if the parent does not exist it means that it has been evicted already. We can ignore it.
// 			continue
// 		}
// 		conflictParents.Add(parent)
// 	}
//
// 	return true
// }
