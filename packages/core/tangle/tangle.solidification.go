package tangle

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/walker"
)

func (t *Tangle) becameSolidOrInvalid(metadata *BlockMetadata) (becameSolid, becameInvalid bool) {
	metadata.Lock()
	defer metadata.Unlock()

	if metadata.solid || metadata.invalid {
		return
	}

	metadata.unsolidParentsCounter, metadata.invalid = t.checkParents(metadata)
	if !metadata.invalid && metadata.unsolidParentsCounter == 0 {
		metadata.solid = true
	}
	// only one of the returned values can be true
	return metadata.solid, metadata.invalid
}

func (t *Tangle) checkParents(metadata *BlockMetadata) (unsolidParents uint8, anyParentInvalid bool) {
	for parentID := range metadata.ParentIDs() {
		parentMetadata, exists := t.blockMetadata(parentID)
		if !exists {
			// Should never happen as parent's metadata is always created before this point.
			panic(errors.Errorf("parent block %s of block %s does not exist", parentID, metadata.id))
		}

		parentMetadata.RLock()
		if parentMetadata.invalid {
			parentMetadata.RUnlock()
			return unsolidParents, true
		}
		if parentMetadata.missing || !parentMetadata.solid {
			unsolidParents++
		}
		parentMetadata.RUnlock()
	}

	return unsolidParents, false
}

func (t *Tangle) propagateSolidityToChildren(metadata *BlockMetadata) {
	for propagationWalker := walker.New[*BlockMetadata](true).Push(metadata); propagationWalker.HasNext(); {
		for _, childMetadata := range propagationWalker.Next().Children() {
			if t.decreaseUnsolidParentsCounter(childMetadata) {
				t.Events.BlockSolid.Trigger(childMetadata)

				propagationWalker.Push(childMetadata)
			}
		}
	}
}

func (t *Tangle) propagateInvalidityToChildren(metadata *BlockMetadata) {
	for propagationWalker := walker.New[*BlockMetadata](true).Push(metadata); propagationWalker.HasNext(); {
		for _, childMetadata := range propagationWalker.Next().Children() {
			if childMetadata.setInvalid() {
				t.Events.BlockInvalid.Trigger(childMetadata)

				propagationWalker.Push(childMetadata)
			}
		}
	}
}

func (t *Tangle) decreaseUnsolidParentsCounter(metadata *BlockMetadata) bool {
	metadata.Lock()
	defer metadata.Unlock()

	if !metadata.Initialized() {
		return false
	}

	metadata.unsolidParentsCounter--
	metadata.solid = metadata.unsolidParentsCounter == 0

	return metadata.solid
}
