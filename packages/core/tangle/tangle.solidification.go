package tangle

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/walker"
)

type RWLock interface {
	RLock()
	RUnlock()
	Lock()
	Unlock()
}

type Solidifier struct {
	tangle *Tangle

	lockProvider func(blockID BlockID) RWLock

	childDependencies          map[BlockID][]*BlockMetadata
	unsolidDependenciesCounter map[BlockID]uint8

	unsolidDependenciesCounterMutex sync.Mutex
	childDependenciesMutex          sync.Mutex
}

func NewSolidifier() {

}

func (s *Solidifier) lockEntity(metadata *BlockMetadata) {
	for parentID := range metadata.ParentIDs() {
		s.lockProvider(parentID).RLock()
	}

	metadata.Lock()
}

func (s *Solidifier) unlockEntity(metadata *BlockMetadata) {
	for parentID := range metadata.ParentIDs() {
		s.blockMetadata(parentID).RUnlock()
	}

	metadata.Unlock()
}

func (s *Solidifier) blockMetadata(blockID BlockID) *BlockMetadata {
	if parentMetadata, exists := s.tangle.blockMetadata(blockID); !exists {
		return parentMetadata
	}

	panic(errors.Errorf("block %s does not exist", blockID))
}

func (t *Solidifier) solidify(metadata *BlockMetadata) {
	becameSolid, becameInvalid := t.becameSolidOrInvalid(metadata)
	if becameInvalid {
		t.propagateInvalidityToChildren(metadata)
	}

	if becameSolid {
		t.propagateSolidityToChildren(metadata)
	}
}

func (t *Solidifier) becameSolidOrInvalid(metadata *BlockMetadata) (becameSolid, becameInvalid bool) {
	t.lockEntity(metadata)
	defer t.unlockEntity(metadata)

	t.unsolidDependenciesCounterMutex.Lock()
	t.unsolidDependenciesCounter[metadata.id], metadata.invalid = t.checkParents(metadata)
	if !metadata.invalid && t.unsolidDependenciesCounter[metadata.id] == 0 {
		metadata.solid = true
	}

	t.unsolidDependenciesCounterMutex.Unlock()

	if metadata.invalid {
		t.Events.BlockInvalid.Trigger(metadata)
	}

	if metadata.solid {
		t.Events.BlockSolid.Trigger(metadata)
	}

	// only one of the returned values can be true
	return metadata.solid, metadata.invalid
}

func (t *Solidifier) checkParents(metadata *BlockMetadata) (unsolidParents uint8, anyParentInvalid bool) {
	for parentID := range metadata.ParentIDs() {
		parentMetadata, exists := t.blockMetadata(parentID)
		if !exists {
			// Should never happen as parent's metadata is always created before this point.
			panic(errors.Errorf("parent block %s of block %s does not exist", parentID, metadata.id))
		}

		if parentMetadata.invalid {
			return unsolidParents, true
		}
		if parentMetadata.missing || !parentMetadata.solid {
			t.childDependenciesMutex.Lock()
			t.childDependencies[parentID] = append(t.childDependencies[parentID], metadata)
			t.childDependenciesMutex.Unlock()

			unsolidParents++
		}
	}

	return unsolidParents, false
}

func (t *Solidifier) propagateSolidityToChildren(metadata *BlockMetadata) {
	for propagationWalker := walker.New[*BlockMetadata](true).Push(metadata); propagationWalker.HasNext(); {
		childID := propagationWalker.Next().id

		t.childDependenciesMutex.Lock()
		deps := t.childDependencies[childID]
		delete(t.childDependencies, childID)
		t.childDependenciesMutex.Unlock()

		for _, childMetadata := range deps {
			fmt.Println("propagating solidity to child", childMetadata.id, "from", metadata.id)
			if t.decreaseUnsolidParentsCounter(childMetadata) {
				propagationWalker.Push(childMetadata)
			}
		}
	}
}

func (t *Solidifier) propagateInvalidityToChildren(metadata *BlockMetadata) {
	for propagationWalker := walker.New[*BlockMetadata](true).Push(metadata); propagationWalker.HasNext(); {
		for _, childMetadata := range propagationWalker.Next().Children() {
			if childMetadata.setInvalid() {
				t.Events.BlockInvalid.Trigger(childMetadata)

				propagationWalker.Push(childMetadata)
			}
		}
	}
}

func (t *Solidifier) decreaseUnsolidParentsCounter(metadata *BlockMetadata) bool {
	metadata.Lock()
	defer metadata.Unlock()

	t.unsolidDependenciesCounterMutex.Lock()
	t.unsolidDependenciesCounter[metadata.id]--
	metadata.solid = t.unsolidDependenciesCounter[metadata.id] == 0
	fmt.Println("decreasing unsolid parents counter of", metadata.id, "to", t.unsolidDependenciesCounter[metadata.id], "solid flag", metadata.solid)

	if metadata.solid && t.unsolidDependenciesCounter[metadata.id] != 0 {
		fmt.Println("co kurwa", t.unsolidDependenciesCounter[metadata.id])
	}
	t.unsolidDependenciesCounterMutex.Unlock()

	if !metadata.solid {
		return false
	}

	t.Events.BlockSolid.Trigger(metadata)

	return true
}
