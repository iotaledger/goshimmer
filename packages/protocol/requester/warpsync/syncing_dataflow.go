package warpsync

import (
	"context"

	"github.com/celestiaorg/smt"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/dataflow"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
)

// syncingFlowParams is a container for parameters to be used in the warpsyncing of a slot.
type syncingFlowParams struct {
	ctx            context.Context
	targetSlot     slot.Index
	targetEC       commitment.ID
	targetPrevEC   commitment.ID
	slotChannels   *slotChannels
	neighbor       *p2p.Neighbor
	tangleTree     *smt.SparseMerkleTree
	slotBlocksLeft int64
	slotBlocks     map[models.BlockID]*models.Block
	roots          *commitment.Roots
}

func (m *Manager) slotStartCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	select {
	case slotStart, ok := <-params.slotChannels.startChan:
		if !ok {
			return nil
		}
		if valid, err := isOnTargetChain(slotStart.si, slotStart.sc, params); !valid {
			return errors.Wrap(err, "received invalid slot start")
		}

		params.slotBlocksLeft = slotStart.blocksCount
		m.log.Debugw("read slot block count", "Index", slotStart.si, "blocksCount", params.slotBlocksLeft)
	case <-params.ctx.Done():
		return errors.Errorf("canceled while receiving slot %d start: %s", params.targetSlot, params.ctx.Err())
	}

	return next(params)
}

func (m *Manager) slotBlockCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	for {
		if params.slotBlocksLeft == 0 {
			m.log.Debugf("all blocks for slot %d received", params.targetSlot)
			break
		}
		select {
		case slotBlock, ok := <-params.slotChannels.blockChan:
			if !ok {
				return nil
			}
			if valid, err := isOnTargetChain(slotBlock.si, slotBlock.sc, params); !valid {
				return errors.Wrap(err, "received invalid block")
			}

			block := slotBlock.block
			if _, exists := params.slotBlocks[block.ID()]; exists {
				return errors.Errorf("received duplicate block %s for slot %d", block.ID(), params.targetSlot)
			}

			m.log.Debugw("read block", "peer", params.neighbor, "Index", slotBlock.si, "blockID", block.ID())

			if _, err := params.tangleTree.Update(lo.PanicOnErr(block.ID().Bytes()), lo.PanicOnErr(block.ID().Bytes())); err != nil {
				return errors.Wrap(err, "error updating tangleTree")
			}
			params.slotBlocks[block.ID()] = block
			params.slotBlocksLeft--

			m.log.Debugf("slot %d: %d blocks left", params.targetSlot, params.slotBlocksLeft)
		case <-params.ctx.Done():
			return errors.Errorf("canceled while receiving blocks for slot %d: %s", params.targetSlot, params.ctx.Err())
		}
	}

	return next(params)
}

func (m *Manager) slotEndCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	select {
	case slotEnd, ok := <-params.slotChannels.endChan:
		if !ok {
			return nil
		}
		if valid, err := isOnTargetChain(slotEnd.si, slotEnd.sc, params); !valid {
			return errors.Wrap(err, "received invalid slot end")
		}

		params.roots = slotEnd.roots

		m.log.Debugw("read slot end", "Index", params.targetSlot)
	case <-params.ctx.Done():
		return errors.Errorf("canceled while ending slot %d: %s", params.targetSlot, params.ctx.Err())
	}

	return next(params)
}

func (m *Manager) slotVerifyCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	// rootID := commitment.NewRootsID(commitment.NewMerkleRoot(params.tangleTree.Root()), params.stateMutationRoot, params.stateRoot, params.manaRoot)
	//
	// syncedECRecord := commitment.New(commitment.NewID(params.targetSlot, rootID, params.targetPrevEC))
	// syncedECRecord.PublishData(params.targetPrevEC, params.targetSlot, rootID)
	//
	// if syncedECRecord.ID() != params.targetEC {
	// 	return errors.Errorf("slot %d SlotCommitment record is not correct", params.targetSlot)
	// }

	return next(params)
}

func (m *Manager) slotProcessBlocksCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	for _, blk := range params.slotBlocks {
		m.blockProcessorFunc(params.neighbor, blk)
	}

	return next(params)
}

func isOnTargetChain(ei slot.Index, ec commitment.ID, params *syncingFlowParams) (valid bool, err error) {
	if ei != params.targetSlot {
		return false, errors.Errorf("received slot %d while we expected slot %d", ei, params.targetSlot)
	}
	if ec != params.targetEC {
		return false, errors.Errorf("received on wrong SlotCommitment chain for slot %d", params.targetSlot)
	}

	return true, nil
}
