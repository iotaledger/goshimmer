package warpsync

import (
	"context"
	"fmt"

	"github.com/celestiaorg/smt"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"
)

type epochSyncBlock struct {
	ei               epoch.Index
	ec               epoch.EC
	block            *tangle.Block
	peer             *peer.Peer
	epochBlocksCount int64
}

type epochSyncEnd struct {
	ei                epoch.Index
	ec                epoch.EC
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
	manaRoot          epoch.MerkleRoot
}

type blockReceived struct {
	block *tangle.Block
	peer  *peer.Peer
}

func (m *Manager) SyncRange(ctx context.Context, start, end epoch.Index, startEC epoch.EC, ecChain map[epoch.Index]*epoch.ECRecord) error {
	if m.syncingInProgress {
		return fmt.Errorf("epoch syncing already in progress")
	}

	if m.isStopped {
		return fmt.Errorf("warpsync manager is stopped")
	}

	m.syncingInProgress = true
	defer func() { m.syncingInProgress = false }()

	epochBlocks := make(map[epoch.Index][]*blockReceived)
	tangleRoots := make(map[epoch.Index]*smt.SparseMerkleTree)
	epochSyncEnds := make(map[epoch.Index]*epochSyncEnd)

	// We do not request the start nor the ending epoch, as we know the beginning and the end of the chain.
	for ei := end - 1; ei > start; ei-- {
		db, _ := database.NewMemDB()
		tangleRoots[ei] = smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))
		m.requestEpochBlocks(ei, epoch.ComputeEC(ecChain[ei]))
	}

	// Counters for the number of blocks received and the number of blocks in the epoch.
	epochsToReceive := end - start - 1
	epochBlocksToReceive := make(map[epoch.Index]int64)
	epochBlocksReceived := make(map[epoch.Index]map[tangle.BlockID]types.Empty)

blocksReadLoop:
	for {
		select {
		case epochSyncBlock := <-m.epochSyncBlockChan:
			ei := epochSyncBlock.ei
			if ei <= start || ei > end-1 {
				m.log.Warnf("received block for epoch %d while we are syncing from %d to %d", ei, start, end)
				continue
			}
			if epochSyncBlock.ec != epoch.ComputeEC(ecChain[ei]) {
				m.log.Warnf("received block on wrong EC chain")
				continue
			}
			if _, exists := epochSyncEnds[ei]; exists {
				m.log.Warnf("received block for already-ended epoch %d", ei)
				continue
			}
			if epochBlocks[ei] == nil {
				epochBlocks[ei] = make([]*blockReceived, 0)
				epochBlocksReceived[ei] = make(map[tangle.BlockID]types.Empty)
			}

			block := epochSyncBlock.block
			if _, exists := epochBlocksReceived[ei][block.ID()]; exists {
				continue
			}
			if _, exists := epochBlocksToReceive[ei]; !exists {
				epochBlocksToReceive[ei] = epochSyncBlock.epochBlocksCount
			}

			m.log.Debugw("read block", "peer", epochSyncBlock.peer.ID(), "EI", ei, "blockID", block.ID())

			tangleRoots[ei].Update(block.IDBytes(), block.IDBytes())
			epochBlocks[ei] = append(epochBlocks[ei], &blockReceived{
				block: block,
				peer:  epochSyncBlock.peer,
			})

			epochBlocksReceived[ei][block.ID()] = types.Void
			epochBlocksToReceive[ei]--
			if epochBlocksToReceive[ei] == 0 {
				epochsToReceive--
				if epochsToReceive == 0 {
					break blocksReadLoop
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("cancelled while syncing epoch range %d to %d: %s", start, end, ctx.Err())
		}
	}

terminatorsReadLoop:
	for {
		select {
		case epochSyncEnd := <-m.epochSyncEndChan:
			ei := epochSyncEnd.ei
			ec := epochSyncEnd.ec
			if ei <= start || ei > end-1 {
				m.log.Warnf("received epoch terminator for epoch %d while we are syncing from %d to %d", ei, start, end)
				continue
			}
			if ec != epoch.ComputeEC(ecChain[ei]) {
				m.log.Warnf("received epoch terminator on wrong EC chain")
				continue
			}
			if _, exists := epochSyncEnds[ei]; exists {
				m.log.Warnf("received termination for already-ended epoch %d", ei)
				continue
			}

			epochSyncEnds[ei] = epochSyncEnd
			m.log.Debugw("read epoch end", "EI", ei)

			epochsToReceive--
			m.log.Debugf("epochs left %d", epochsToReceive)
			if epochsToReceive == 0 {
				break terminatorsReadLoop
			}
		case <-ctx.Done():
			return fmt.Errorf("cancelled while syncing epoch range %d to %d: %s", start, end, ctx.Err())
		}
	}

	// Verify the epochs.
	for ei := end - 1; ei > start; ei-- {
		tangleRoot := tangleRoots[ei].Root()
		epochSyncEnd := epochSyncEnds[ei]
		ecRecord := epoch.NewECRecord(ei)
		ecRecord.SetECR(epoch.ComputeECR(
			epoch.NewMerkleRoot(tangleRoot),
			epochSyncEnd.stateMutationRoot,
			epochSyncEnd.stateRoot,
			epochSyncEnd.manaRoot,
		))

		if ei == start+1 {
			ecRecord.SetPrevEC(startEC)
		} else {
			ecRecord.SetPrevEC(epoch.ComputeEC(ecChain[ei-1]))
		}

		if epoch.ComputeEC(ecChain[ei]) != epoch.ComputeEC(ecRecord) {
			return fmt.Errorf("epoch %d EC record is not correct", ei)
		}
	}

	// We verified the epochs contents for the range, we can now parse them.
	for ei := start; ei <= end; ei++ {
		for _, blockReceived := range epochBlocks[ei] {
			blockBytes, err := blockReceived.block.Bytes()
			if err != nil {
				m.log.Warnf("received block from %s failed to serialize: %s", blockReceived.peer.ID(), err)
			}
			m.tangle.ProcessGossipBlock(blockBytes, blockReceived.peer)
		}
	}

	m.log.Debugw("sync successful")

	return nil
}

func (m *Manager) requestEpochBlocks(ei epoch.Index, ec epoch.EC) {
	epochBlocksReq := &wp.EpochBlocksRequest{
		EI: int64(ei),
		EC: ec.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksRequest{EpochBlocksRequest: epochBlocksReq}}
	m.p2pManager.Send(packet, protocolID)

	m.log.Debugw("sent epoch blocks request", "EI", ei, "EC", ec.Base58())
}

func (m *Manager) processEpochBlocksRequestPacket(packetEpochRequest *wp.Packet_EpochBlocksRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochBlocksRequest.GetEI())
	ec := epoch.NewMerkleRoot(packetEpochRequest.EpochBlocksRequest.GetEC())

	m.log.Debugw("received epoch blocks request", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec)

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists || ec != epoch.ComputeEC(ecRecord) {
		m.log.Debugw("epoch blocks request rejected: unknown epoch or mismatching EC", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec)
		return
	}
	blockIDs := epochstorage.GetEpochBlockIDs(ei)
	epochBlocksCount := len(blockIDs)

	// Send epoch's blocks in batches.
	for batchNum := 0; batchNum <= len(blockIDs)/m.blockBatchSize; batchNum++ {
		blocksBytes := make([][]byte, 0)
		for i := batchNum * m.blockBatchSize; i < len(blockIDs) && i < (batchNum+1)*m.blockBatchSize; i++ {
			m.tangle.Storage.Block(blockIDs[i]).Consume(func(block *tangle.Block) {
				blockBytes, err := block.Bytes()
				if err != nil {
					m.log.Errorf("failed to serialize block %s: %s", block.ID().String(), err)
					return
				}
				blocksBytes = append(blocksBytes, blockBytes)
			})
		}

		blocksRes := &wp.EpochBlocks{
			EI:               int64(ei),
			EC:               ec.Bytes(),
			Blocks:           blocksBytes,
			EpochBlocksCount: int64(epochBlocksCount),
		}
		packet := &wp.Packet{Body: &wp.Packet_EpochBlocks{EpochBlocks: blocksRes}}

		m.p2pManager.Send(packet, protocolID, nbr.ID())

		m.log.Debugw("sent epoch blocks batch", "peer", nbr.Peer.ID(), "EI", ei, "blocksLen", len(blocksBytes))
	}

	// Send epoch termination.
	roots := ecRecord.Roots()
	epochBlocksEnd := &wp.EpochBlocksEnd{
		EI:                int64(ei),
		EC:                ec.Bytes(),
		StateMutationRoot: roots.StateMutationRoot.Bytes(),
		StateRoot:         roots.StateRoot.Bytes(),
		ManaRoot:          roots.ManaRoot.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksEnd{EpochBlocksEnd: epochBlocksEnd}}
	m.p2pManager.Send(packet, protocolID, nbr.ID())

	m.log.Debugw("sent epoch blocks termination", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec.Base58())
}

func (m *Manager) processEpochBlocksPacket(packetEpochBlocks *wp.Packet_EpochBlocks, nbr *p2p.Neighbor) {
	if !m.syncingInProgress {
		return
	}

	ei := epoch.Index(packetEpochBlocks.EpochBlocks.GetEI())
	blocksBytes := packetEpochBlocks.EpochBlocks.GetBlocks()
	m.log.Debugw("received epoch blocks", "peer", nbr.Peer.ID(), "EI", ei, "blocksLen", len(blocksBytes))

	for _, blockBytes := range blocksBytes {
		block := new(tangle.Block)
		if err := block.FromBytes(blockBytes); err != nil {
			m.log.Errorw("failed to deserialize block", "peer", nbr.Peer.ID(), "err", err)
			return
		}
		m.epochSyncBlockChan <- &epochSyncBlock{
			ei:               ei,
			ec:               epoch.NewMerkleRoot(packetEpochBlocks.EpochBlocks.GetEC()),
			peer:             nbr.Peer,
			block:            block,
			epochBlocksCount: packetEpochBlocks.EpochBlocks.GetEpochBlocksCount(),
		}

		m.log.Debugw("write block", "blockID", block.ID())
	}
}

func (m *Manager) processEpochBlocksEndPacket(packetEpochBlocksEnd *wp.Packet_EpochBlocksEnd, nbr *p2p.Neighbor) {
	if !m.syncingInProgress {
		return
	}

	ei := epoch.Index(packetEpochBlocksEnd.EpochBlocksEnd.GetEI())
	m.log.Debugw("received epoch blocks end", "peer", nbr.Peer.ID(), "EI", ei)

	m.epochSyncEndChan <- &epochSyncEnd{
		ei:                ei,
		ec:                epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetEC()),
		stateMutationRoot: epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateMutationRoot()),
		stateRoot:         epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateRoot()),
		manaRoot:          epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetManaRoot()),
	}
}
