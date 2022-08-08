package tangleold

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

// maxParentsTimeDifference defines the smallest allowed time difference between a child Block and its parents.
const minParentsTimeDifference = 0 * time.Second

// maxParentsTimeDifference defines the biggest allowed time difference between a child Block and its parents.
const maxParentsTimeDifference = 30 * time.Minute

// region Solidifier ///////////////////////////////////////////////////////////////////////////////////////////////////

// Solidifier is the Tangle's component that solidifies blocks.
type Solidifier struct {
	// Events contains the Solidifier related events.
	Events *SolidifierEvents

	mutex  *syncutils.DAGMutex[BlockID]
	tangle *Tangle
}

// NewSolidifier is the constructor of the Solidifier.
func NewSolidifier(tangle *Tangle) (solidifier *Solidifier) {
	solidifier = &Solidifier{
		Events: newSolidifierEvents(),
		mutex:  syncutils.NewDAGMutex[BlockID](),
		tangle: tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Solidifier) Setup() {
	s.tangle.Storage.Events.BlockStored.Hook(event.NewClosure(func(event *BlockStoredEvent) {
		s.solidify(event.Block)
	}))

	s.Events.BlockSolid.Attach(event.NewClosure(func(event *BlockSolidEvent) {
		s.processChildren(event.Block.ID())
	}))
}

// Solidify solidifies the given Block.
func (s *Solidifier) Solidify(blockID BlockID) {
	s.tangle.Storage.Block(blockID).Consume(s.solidify)
}

// Solidify solidifies the given Block.
func (s *Solidifier) solidify(block *Block) {
	s.tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
		if s.checkBlockSolidity(block, blockMetadata) {
			s.Events.BlockSolid.Trigger(&BlockSolidEvent{block})
		}
	})
}

func (s *Solidifier) processChildren(blockID BlockID) {
	s.tangle.Storage.Children(blockID).Consume(func(child *Child) {
		event.Loop.Submit(func() {
			s.Solidify(child.ChildBlockID())
		})
	})
}

// RetrieveMissingBlock checks if the block is missing and triggers the corresponding events to request it. It returns true if the block has been missing.
func (s *Solidifier) RetrieveMissingBlock(blockID BlockID) (blockWasMissing bool) {
	s.tangle.Storage.BlockMetadata(blockID, func() *BlockMetadata {
		if cachedMissingBlock, stored := s.tangle.Storage.StoreMissingBlock(NewMissingBlock(blockID)); stored {
			cachedMissingBlock.Release()

			blockWasMissing = true
			s.Events.BlockMissing.Trigger(&BlockMissingEvent{blockID})
		}

		return nil
	}).Release()

	return blockWasMissing
}

// checkBlockSolidity checks if the given Block is solid and eventually queues its Children to also be checked.
func (s *Solidifier) checkBlockSolidity(block *Block, blockMetadata *BlockMetadata) (blockBecameSolid bool) {
	s.mutex.Lock(block.ID())
	defer s.mutex.Unlock(block.ID())

	if blockMetadata.IsSolid() {
		return false
	}

	if !s.isBlockSolid(block, blockMetadata) {
		return false
	}

	if !s.areParentBlocksValid(block) {
		if blockMetadata.SetObjectivelyInvalid(true) {
			s.tangle.Events.BlockInvalid.Trigger(&BlockInvalidEvent{BlockID: block.ID(), Error: ErrParentsInvalid})
		}

		return false
	}

	return blockMetadata.SetSolid(true)
}

// isBlockSolid checks if the given Block is solid.
func (s *Solidifier) isBlockSolid(block *Block, blockMetadata *BlockMetadata) (solid bool) {
	if block == nil || block.IsDeleted() || blockMetadata == nil || blockMetadata.IsDeleted() {
		return false
	}

	if blockMetadata.IsSolid() {
		return true
	}

	solid = true
	block.ForEachParent(func(parent Parent) {
		// as missing blocks are requested in isBlockMarkedAsSolid, we need to be aware of short-circuit evaluation
		// rules, thus we need to evaluate isBlockMarkedAsSolid !!first!!
		solid = s.isBlockMarkedAsSolid(parent.ID) && solid
	})

	return
}

// isBlockMarkedAsSolid checks whether the given block is solid and marks it as missing if it isn't known.
func (s *Solidifier) isBlockMarkedAsSolid(blockID BlockID) (solid bool) {
	if blockID == EmptyBlockID {
		return true
	}

	if s.RetrieveMissingBlock(blockID) {
		return false
	}

	s.tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *BlockMetadata) {
		solid = blockMetadata.IsSolid()
	})

	return
}

// areParentBlocksValid checks whether the parents of the given Block are valid.
func (s *Solidifier) areParentBlocksValid(block *Block) (valid bool) {
	valid = true
	block.ForEachParent(func(parent Parent) {
		if !valid {
			return
		}

		if parent.ID == EmptyBlockID {
			if s.tangle.Options.GenesisNode != nil {
				valid = *s.tangle.Options.GenesisNode == block.IssuerPublicKey()
				return
			}

			s.tangle.Storage.BlockMetadata(parent.ID).Consume(func(blockMetadata *BlockMetadata) {
				timeDifference := block.IssuingTime().Sub(blockMetadata.SolidificationTime())
				if valid = timeDifference >= minParentsTimeDifference && timeDifference <= maxParentsTimeDifference; !valid {
					return
				}
			})

			return
		}

		s.tangle.Storage.Block(parent.ID).Consume(func(parentBlock *Block) {
			if parent.Type == ShallowLikeParentType {
				if _, valid = parentBlock.Payload().(utxo.Transaction); !valid {
					return
				}
			}

			if valid = s.isParentBlockValid(parentBlock, block); !valid {
				return
			}
		})
	})

	return
}

// isParentBlockValid checks whether the given parent Block is valid.
func (s *Solidifier) isParentBlockValid(parentBlock *Block, childBlock *Block) (valid bool) {
	timeDifference := childBlock.IssuingTime().Sub(parentBlock.IssuingTime())
	if timeDifference < minParentsTimeDifference || timeDifference > maxParentsTimeDifference {
		return false
	}

	s.tangle.Storage.BlockMetadata(parentBlock.ID()).Consume(func(blockMetadata *BlockMetadata) {
		valid = !blockMetadata.IsObjectivelyInvalid()
	})

	return valid
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
