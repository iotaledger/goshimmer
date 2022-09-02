package warpsync

import (
	"context"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/hive.go/core/generics/dataflow"
	"github.com/iotaledger/hive.go/core/identity"
)

// syncingFlowParams is a container for parameters to be used in the warpsyncing of an epoch.
type syncingFlowParams struct {
	ctx               context.Context
	targetEpoch       epoch.Index
	targetEC          epoch.EC
	targetPrevEC      epoch.EC
	epochChannels     *epochChannels
	peerID            identity.ID
	tangleTree        *smt.SparseMerkleTree
	epochBlocksLeft   int64
	epochBlocks       map[tangleold.BlockID]*tangleold.Block
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
	manaRoot          epoch.MerkleRoot
}

func (m *Manager) epochStartCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	select {
	case epochStart, ok := <-params.epochChannels.startChan:
		if !ok {
			return nil
		}
		if valid, err := isOnTargetChain(epochStart.ei, epochStart.ec, params); !valid {
			return errors.Wrap(err, "received invalid epoch start")
		}

		params.epochBlocksLeft = epochStart.blocksCount
		m.log.Debugw("read epoch block count", "EI", epochStart.ei, "blocksCount", params.epochBlocksLeft)
	case <-params.ctx.Done():
		return errors.Errorf("cancelled while receiving epoch %d start: %s", params.targetEpoch, params.ctx.Err())
	}

	return next(params)
}

func (m *Manager) epochBlockCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	for {
		if params.epochBlocksLeft == 0 {
			m.log.Debugf("all blocks for epoch %d received", params.targetEpoch)
			break
		}
		select {
		case epochBlock, ok := <-params.epochChannels.blockChan:
			if !ok {
				return nil
			}
			if valid, err := isOnTargetChain(epochBlock.ei, epochBlock.ec, params); !valid {
				return errors.Wrap(err, "received invalid block")
			}

			block := epochBlock.block
			if _, exists := params.epochBlocks[block.ID()]; exists {
				return errors.Errorf("received duplicate block %s for epoch %d", block.ID(), params.targetEpoch)
			}

			m.log.Debugw("read block", "peer", params.peerID, "EI", epochBlock.ei, "blockID", block.ID())

			params.tangleTree.Update(block.IDBytes(), block.IDBytes())
			params.epochBlocks[block.ID()] = block
			params.epochBlocksLeft--

			m.log.Debugf("epoch %d: %d blocks left", params.targetEpoch, params.epochBlocksLeft)
		case <-params.ctx.Done():
			return errors.Errorf("cancelled while receiving blocks for epoch %d: %s", params.targetEpoch, params.ctx.Err())
		}
	}

	return next(params)
}

func (m *Manager) epochEndCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	select {
	case epochEnd, ok := <-params.epochChannels.endChan:
		if !ok {
			return nil
		}
		if valid, err := isOnTargetChain(epochEnd.ei, epochEnd.ec, params); !valid {
			return errors.Wrap(err, "received invalid epoch end")
		}

		params.stateMutationRoot = epochEnd.stateMutationRoot
		params.stateRoot = epochEnd.stateRoot
		params.manaRoot = epochEnd.manaRoot

		m.log.Debugw("read epoch end", "EI", params.targetEpoch)
	case <-params.ctx.Done():
		return errors.Errorf("cancelled while ending epoch %d: %s", params.targetEpoch, params.ctx.Err())
	}

	return next(params)
}

func (m *Manager) epochVerifyCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	syncedECRecord := epoch.NewECRecord(params.targetEpoch)
	syncedECRecord.SetECR(epoch.ComputeECR(
		epoch.NewMerkleRoot(params.tangleTree.Root()),
		params.stateMutationRoot,
		params.stateRoot,
		params.manaRoot,
	))
	syncedECRecord.SetPrevEC(params.targetPrevEC)

	if syncedECRecord.ComputeEC() != params.targetEC {
		return errors.Errorf("epoch %d EC record is not correct", params.targetEpoch)
	}

	return next(params)
}

func (m *Manager) epochProcessBlocksCommand(params *syncingFlowParams, next dataflow.Next[*syncingFlowParams]) (err error) {
	for _, blk := range params.epochBlocks {
		neighbors := m.p2pManager.GetNeighborsByID([]identity.ID{params.peerID})
		if len(neighbors) != 1 {
			return errors.Errorf("neighbor %s not peered anymore after receiving warpsynced block")
		}
		m.blockProcessorFunc(blk, neighbors[0].Peer)
	}

	return next(params)
}

func isOnTargetChain(ei epoch.Index, ec epoch.EC, params *syncingFlowParams) (valid bool, err error) {
	if ei != params.targetEpoch {
		return false, errors.Errorf("received epoch %d while we expected epoch %d", ei, params.targetEpoch)
	}
	if ec != params.targetEC {
		return false, errors.Errorf("received on wrong EC chain for epoch %d", params.targetEpoch)
	}

	return true, nil
}
