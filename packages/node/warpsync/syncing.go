package warpsync

import (
	"context"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/dataflow"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/sync/errgroup"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
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

func (m *Manager) syncRange(ctx context.Context, start, end epoch.Index, startEC epoch.EC, ecChain map[epoch.Index]epoch.EC, validPeers *set.AdvancedSet[identity.ID]) (completedEpoch epoch.Index, err error) {
	startRange := start + 1
	endRange := end - 1

	m.startSyncing(startRange, endRange)
	defer m.endSyncing()

	eg, errCtx := errgroup.WithContext(ctx)
	eg.SetLimit(m.concurrency)

	epochProcessedChan := make(chan epoch.Index)
	discardedPeers := set.NewAdvancedSet[identity.ID]()

	workerFunc := m.syncEpochFunc(errCtx, eg, validPeers, discardedPeers, ecChain, epochProcessedChan)
	completedEpoch = m.queueSlidingEpochs(errCtx, startRange, endRange, workerFunc, epochProcessedChan)

	if err := eg.Wait(); err != nil {
		return completedEpoch, errors.Wrapf(err, "sync failed for range %d-%d", start, end)
	}
	return completedEpoch, nil
}

func (m *Manager) syncEpochFunc(errCtx context.Context, eg *errgroup.Group, validPeers *set.AdvancedSet[identity.ID], discardedPeers *set.AdvancedSet[identity.ID], ecChain map[epoch.Index]epoch.EC, epochProcessedChan chan epoch.Index) func(targetEpoch epoch.Index) {
	return func(targetEpoch epoch.Index) {
		eg.Go(func() error {
			success := false
			for it := validPeers.Iterator(); it.HasNext() && !success; {
				peerID := it.Next()
				if discardedPeers.Has(peerID) {
					m.log.Debugw("skipping discarded peer", "peer", peerID)
					continue
				}

				db, _ := database.NewMemDB()
				tangleTree := smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))

				epochChannels := m.startEpochSyncing(targetEpoch)
				epochChannels.RLock()

				m.requestEpochBlocks(targetEpoch, ecChain[targetEpoch], peerID)

				dataflow.New(
					m.epochStartCommand,
					m.epochBlockCommand,
					m.epochEndCommand,
					m.epochVerifyCommand,
					m.epochProcessBlocksCommand,
				).WithTerminationCallback(func(params *syncingFlowParams) {
					params.epochChannels.RUnlock()
					m.endEpochSyncing(params.targetEpoch)
				}).WithSuccessCallback(func(params *syncingFlowParams) {
					success = true
					select {
					case <-params.ctx.Done():
						return
					case epochProcessedChan <- params.targetEpoch:
					}
					m.log.Infow("synced epoch", "epoch", params.targetEpoch, "peer", params.peerID)
				}).WithErrorCallback(func(flowErr error, params *syncingFlowParams) {
					discardedPeers.Add(params.peerID)
					m.log.Warnf("error while syncing epoch %d from peer %s: %s", params.targetEpoch, params.peerID, flowErr)
				}).Run(&syncingFlowParams{
					ctx:           errCtx,
					targetEpoch:   targetEpoch,
					targetEC:      ecChain[targetEpoch],
					targetPrevEC:  ecChain[targetEpoch-1],
					epochChannels: epochChannels,
					peerID:        peerID,
					tangleTree:    tangleTree,
					epochBlocks:   make(map[tangleold.BlockID]*tangleold.Block),
				})
			}

			if !success {
				return errors.Errorf("unable to sync epoch %d", targetEpoch)
			}

			return nil
		})
	}
}

func (m *Manager) queueSlidingEpochs(errCtx context.Context, startRange, endRange epoch.Index, workerFunc func(epoch.Index), epochProcessedChan chan epoch.Index) (completedEpoch epoch.Index) {
	processedEpochs := make(map[epoch.Index]types.Empty)
	for ei := startRange; ei < startRange+epoch.Index(m.concurrency) && ei <= endRange; ei++ {
		workerFunc(ei)
	}

	windowStart := startRange
	for {
		select {
		case processedEpoch := <-epochProcessedChan:
			processedEpochs[processedEpoch] = types.Void
			for {
				if _, processed := processedEpochs[windowStart]; processed {
					completedEpoch = windowStart
					if completedEpoch == endRange {
						return
					}
					windowEnd := windowStart + epoch.Index(m.concurrency)
					if windowEnd <= endRange {
						workerFunc(windowEnd)
					}
					windowStart++
				} else {
					break
				}
			}
		case <-errCtx.Done():
			return
		}
	}
}

func (m *Manager) startSyncing(startRange, endRange epoch.Index) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	m.syncingInProgress = true
	m.epochsChannels = make(map[epoch.Index]*epochChannels)
	for ei := startRange; ei <= endRange; ei++ {
		m.epochsChannels[ei] = &epochChannels{}
	}
}

func (m *Manager) endSyncing() {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	m.syncingInProgress = false
	m.epochsChannels = nil
}

func (m *Manager) startEpochSyncing(ei epoch.Index) (epochChannels *epochChannels) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	epochChannels = m.epochsChannels[ei]

	epochChannels.Lock()
	defer epochChannels.Unlock()

	epochChannels.startChan = make(chan *epochSyncStart, 1)
	epochChannels.blockChan = make(chan *epochSyncBlock, 1)
	epochChannels.endChan = make(chan *epochSyncEnd, 1)
	epochChannels.stopChan = make(chan struct{})
	epochChannels.active = true

	return
}

func (m *Manager) endEpochSyncing(ei epoch.Index) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	epochChannels := m.epochsChannels[ei]

	epochChannels.active = false
	close(epochChannels.stopChan)
	epochChannels.Lock()
	defer epochChannels.Unlock()

	close(epochChannels.startChan)
	close(epochChannels.blockChan)
	close(epochChannels.endChan)
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
	m.log.Debugw("sent epoch start", "peer", nbr.Peer.ID(), "EI", ei, "blocksCount", blocksCount)

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
	m.log.Debugw("sent epoch blocks end", "peer", nbr.ID(), "EI", ei, "EC", ec.Base58())
}

func (m *Manager) processEpochBlocksStartPacket(packetEpochBlocksStart *wp.Packet_EpochBlocksStart, nbr *p2p.Neighbor) {
	epochBlocksStart := packetEpochBlocksStart.EpochBlocksStart
	ei := epoch.Index(epochBlocksStart.GetEI())

	epochChannels := m.getEpochChannels(ei)
	if epochChannels == nil {
		return
	}

	epochChannels.RLock()
	defer epochChannels.RUnlock()

	if !epochChannels.active {
		return
	}

	m.log.Debugw("received epoch blocks start", "peer", nbr.Peer.ID(), "EI", ei, "blocksCount", epochBlocksStart.GetBlocksCount())

	epochChannels.startChan <- &epochSyncStart{
		ei:          ei,
		ec:          epoch.NewMerkleRoot(epochBlocksStart.GetEC()),
		blocksCount: epochBlocksStart.GetBlocksCount(),
	}
}

func (m *Manager) processEpochBlocksBatchPacket(packetEpochBlocksBatch *wp.Packet_EpochBlocksBatch, nbr *p2p.Neighbor) {
	epochBlocksBatch := packetEpochBlocksBatch.EpochBlocksBatch
	ei := epoch.Index(epochBlocksBatch.GetEI())

	epochChannels := m.getEpochChannels(ei)
	if epochChannels == nil {
		return
	}

	epochChannels.RLock()
	defer epochChannels.RUnlock()

	if !epochChannels.active {
		return
	}

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
	}
}

func (m *Manager) processEpochBlocksEndPacket(packetEpochBlocksEnd *wp.Packet_EpochBlocksEnd, nbr *p2p.Neighbor) {
	epochBlocksBatch := packetEpochBlocksEnd.EpochBlocksEnd
	ei := epoch.Index(epochBlocksBatch.GetEI())

	epochChannels := m.getEpochChannels(ei)
	if epochChannels == nil {
		return
	}

	epochChannels.RLock()
	defer epochChannels.RUnlock()

	if !epochChannels.active {
		return
	}

	m.log.Debugw("received epoch blocks end", "peer", nbr.Peer.ID(), "EI", ei)

	epochChannels.endChan <- &epochSyncEnd{
		ei:                ei,
		ec:                epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetEC()),
		stateMutationRoot: epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateMutationRoot()),
		stateRoot:         epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateRoot()),
		manaRoot:          epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetManaRoot()),
	}
}

func (m *Manager) getEpochChannels(ei epoch.Index) *epochChannels {
	m.syncingLock.RLock()
	defer m.syncingLock.RUnlock()

	if !m.syncingInProgress {
		return nil
	}

	return m.epochsChannels[ei]
}
