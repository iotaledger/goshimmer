package warpsync

import (
	"context"
	"sync"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/dataflow"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"golang.org/x/crypto/blake2b"
)

type epochSyncStart struct {
	ei          epoch.Index
	ec          epoch.EC
	blocksCount int64
}

type epochSyncBlock struct {
	ei    epoch.Index
	ec    epoch.EC
	block *tangleold.Block
	peer  *peer.Peer
}

type epochSyncEnd struct {
	ei                epoch.Index
	ec                epoch.EC
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
	manaRoot          epoch.MerkleRoot
}

type blockReceived struct {
	block *tangleold.Block
	peer  *peer.Peer
}

func (m *Manager) syncRange(ctx context.Context, start, end epoch.Index, startEC epoch.EC, ecChain map[epoch.Index]epoch.EC, validPeers *set.AdvancedSet[*peer.Peer]) (err error) {
	if m.IsStopped() {
		return errors.Errorf("warpsync manager is stopped")
	}

	startRange := start + 1
	endRange := end - 1

	m.startSyncing(startRange, endRange)
	defer m.endSyncing()

	var wg sync.WaitGroup
	epochProcessingChan := make(chan epoch.Index)
	epochProcessingStopChan := make(chan struct{})
	discardedPeers := set.NewAdvancedSet[*peer.Peer]()

	// Spawn concurreny amount of goroutines.
	for worker := 0; worker < m.concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				var targetEpoch epoch.Index
				select {
				case targetEpoch = <-epochProcessingChan:
				case <-epochProcessingStopChan:
					return
				case <-ctx.Done():
					return
				}

				success := false
				for it := validPeers.Iterator(); it.HasNext() && !success; {
					peer := it.Next()
					if discardedPeers.Has(peer) {
						m.log.Debugw("skipping discarded peer", "peer", peer.ID())
						continue
					}

					db, _ := database.NewMemDB()
					tangleTree := smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))

					m.startEpochSyncing(targetEpoch)

					m.requestEpochBlocks(targetEpoch, ecChain[targetEpoch], peer.ID())

					dataflow.New(
						m.epochStartCommand,
						m.epochBlockCommand,
						m.epochEndCommand,
						m.epochVerifyCommand,
						m.epochProcessBlocksCommand,
					).WithErrorCallback(func(flowErr error, params *syncingFlowParams) {
						discardedPeers.Add(params.peer)
						m.log.Warnf("error while syncing epoch %d from peer %s: %s", params.targetEpoch, params.peer.ID(), flowErr)
					}).WithTerminationCallback(func(params *syncingFlowParams) {
						m.endEpochSyncing(targetEpoch)
					}).WithSuccessCallback(func(params *syncingFlowParams) {
						success = true
						m.log.Debugw("synced epoch", "epoch", params.targetEpoch, "peer", params.peer.ID())
					}).Run(&syncingFlowParams{
						ctx:           ctx,
						targetEpoch:   targetEpoch,
						targetEC:      ecChain[targetEpoch],
						targetPrevEC:  ecChain[targetEpoch-1],
						epochChannels: m.epochChannels[targetEpoch],
						peer:          peer,
						tangleTree:    tangleTree,
						epochBlocks:   make(map[tangleold.BlockID]*tangleold.Block),
					})
				}

				if !success {
					err = errors.Errorf("unable to sync epoch %d", targetEpoch)
					return
				}
			}
		}()
	}

	for ei := startRange; ei <= endRange; ei++ {
		epochProcessingChan <- ei
	}
	close(epochProcessingStopChan)

	wg.Wait()
	close(epochProcessingChan)

	if err != nil {
		return err
	}

	m.log.Debugf("sync successful for range %d-%d", start, end)

	return nil
}

func (m *Manager) startSyncing(start, end epoch.Index) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	m.syncingInProgress = true
	m.epochChannels = make(map[epoch.Index]*epochChannels)
	for ei := start; ei <= end; ei++ {
		m.epochChannels[ei] = &epochChannels{}
	}
}

func (m *Manager) endSyncing() {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	m.syncingInProgress = false
	m.epochChannels = nil
}

func (m *Manager) startEpochSyncing(ei epoch.Index) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	epochChannels := m.epochChannels[ei]
	epochChannels.Lock()
	defer epochChannels.Unlock()

	epochChannels.startChan = make(chan *epochSyncStart)
	epochChannels.blockChan = make(chan *epochSyncBlock)
	epochChannels.endChan = make(chan *epochSyncEnd)
	epochChannels.stopChan = make(chan struct{})
}

func (m *Manager) endEpochSyncing(ei epoch.Index) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	epochChannels := m.epochChannels[ei]

	close(epochChannels.stopChan)
	epochChannels.Lock()
	defer epochChannels.Unlock()

	close(epochChannels.startChan)
	close(epochChannels.blockChan)
	close(epochChannels.endChan)

	delete(m.epochChannels, ei)
}

func (m *Manager) processEpochBlocksRequestPacket(packetEpochRequest *wp.Packet_EpochBlocksRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochBlocksRequest.GetEI())
	ec := epoch.NewMerkleRoot(packetEpochRequest.EpochBlocksRequest.GetEC())

	m.log.Debugw("received epoch blocks request", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec)

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists || ec != ecRecord.ComputeEC() {
		m.log.Debugw("epoch blocks request rejected: unknown epoch or mismatching EC", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec)
		return
	}
	blockIDs := epochstorage.GetEpochBlockIDs(ei)
	blocksCount := len(blockIDs)

	// Send epoch starter.
	m.sendEpochStarter(ei, ec, blocksCount, nbr.ID())
	m.log.Debugw("sent epoch starter", "peer", nbr.Peer.ID(), "EI", ei, "blocksCount", blocksCount)

	// Send epoch's blocks in batches.
	for batchNum := 0; batchNum <= len(blockIDs)/m.blockBatchSize; batchNum++ {
		blocks := make([]*tangleold.Block, 0)
		for i := batchNum * m.blockBatchSize; i < len(blockIDs) && i < (batchNum+1)*m.blockBatchSize; i++ {
			block, err := m.blockLoaderFunc(blockIDs[i])
			if err != nil {
				m.log.Errorf("failed to load block %s: %s", blockIDs[i], err)
				return
			}
			blocks = append(blocks, block)
		}

		m.sendBlocksBatch(ei, ec, blocks, nbr.ID())
		m.log.Debugw("sent epoch blocks batch", "peer", nbr.ID(), "EI", ei, "blocksLen", len(blocks))
	}

	// Send epoch terminator.
	m.sendEpochEnd(ei, ec, ecRecord.Roots(), nbr.ID())
	m.log.Debugw("sent epoch blocks termination", "peer", nbr.ID(), "EI", ei, "EC", ec.Base58())
}

func (m *Manager) processEpochBlocksStartPacket(packetEpochBlocksStart *wp.Packet_EpochBlocksStart, nbr *p2p.Neighbor) {
	m.syncingLock.RLock()
	defer m.syncingLock.RUnlock()

	if !m.syncingInProgress {
		return
	}

	epochBlocksStart := packetEpochBlocksStart.EpochBlocksStart
	ei := epoch.Index(epochBlocksStart.GetEI())

	epochChannels, exists := m.epochChannels[ei]
	if !exists {
		return
	}

	epochChannels.RLock()
	defer epochChannels.RUnlock()

	m.log.Debugw("received epoch blocks start", "peer", nbr.Peer.ID(), "EI", ei)

	select {
	case <-epochChannels.stopChan:
		return
	case epochChannels.startChan <- &epochSyncStart{
		ei:          ei,
		ec:          epoch.NewMerkleRoot(epochBlocksStart.GetEC()),
		blocksCount: epochBlocksStart.GetBlocksCount(),
	}:
	}
}

func (m *Manager) processEpochBlocksBatchPacket(packetEpochBlocksBatch *wp.Packet_EpochBlocksBatch, nbr *p2p.Neighbor) {
	m.syncingLock.RLock()
	defer m.syncingLock.RUnlock()

	if !m.syncingInProgress {
		return
	}

	epochBlocksBatch := packetEpochBlocksBatch.EpochBlocksBatch
	ei := epoch.Index(epochBlocksBatch.GetEI())

	epochChannels, exists := m.epochChannels[ei]
	if !exists {
		return
	}

	epochChannels.RLock()
	defer epochChannels.RUnlock()

	blocksBytes := epochBlocksBatch.GetBlocks()
	m.log.Debugw("received epoch blocks", "peer", nbr.Peer.ID(), "EI", ei, "blocksLen", len(blocksBytes))

	for _, blockBytes := range blocksBytes {
		block := new(tangleold.Block)
		if err := block.FromBytes(blockBytes); err != nil {
			m.log.Errorw("failed to deserialize block", "peer", nbr.Peer.ID(), "err", err)
			return
		}
		select {
		case <-epochChannels.stopChan:
			return
		case epochChannels.blockChan <- &epochSyncBlock{
			ei:    ei,
			ec:    epoch.NewMerkleRoot(epochBlocksBatch.GetEC()),
			peer:  nbr.Peer,
			block: block,
		}:
		}

		m.log.Debugw("write block", "blockID", block.ID())
	}
}

func (m *Manager) processEpochBlocksEndPacket(packetEpochBlocksEnd *wp.Packet_EpochBlocksEnd, nbr *p2p.Neighbor) {
	m.syncingLock.RLock()
	defer m.syncingLock.RUnlock()

	if !m.syncingInProgress {
		return
	}

	epochBlocksBatch := packetEpochBlocksEnd.EpochBlocksEnd
	ei := epoch.Index(epochBlocksBatch.GetEI())

	epochChannels, exists := m.epochChannels[ei]
	if !exists {
		return
	}

	epochChannels.RLock()
	defer epochChannels.RUnlock()

	m.log.Debugw("received epoch blocks end", "peer", nbr.Peer.ID(), "EI", ei)

	select {
	case <-epochChannels.stopChan:
		return
	case epochChannels.endChan <- &epochSyncEnd{
		ei:                ei,
		ec:                epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetEC()),
		stateMutationRoot: epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateMutationRoot()),
		stateRoot:         epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateRoot()),
		manaRoot:          epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetManaRoot()),
	}:
	}
}
