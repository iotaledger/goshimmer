package warpsync

import (
	"context"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/dataflow"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/sync/errgroup"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type epochSyncStart struct {
	ei          epoch.Index
	ec          commitment.ID
	blocksCount int64
}

type epochSyncBlock struct {
	ei    epoch.Index
	ec    commitment.ID
	block *models.Block
	peer  *peer.Peer
}

type epochSyncEnd struct {
	ei    epoch.Index
	ec    commitment.ID
	roots *commitment.Roots
}

type blockReceived struct {
	block *models.Block
	peer  *peer.Peer
}

func (m *Manager) syncRange(ctx context.Context, start, end epoch.Index, startEC commitment.ID, ecChain map[epoch.Index]commitment.ID, validPeers *orderedmap.OrderedMap[identity.ID, *p2p.Neighbor]) (completedEpoch epoch.Index, err error) {
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

func (m *Manager) syncEpochFunc(errCtx context.Context, eg *errgroup.Group, validPeers *orderedmap.OrderedMap[identity.ID, *p2p.Neighbor], discardedPeers *set.AdvancedSet[identity.ID], ecChain map[epoch.Index]commitment.ID, epochProcessedChan chan epoch.Index) func(targetEpoch epoch.Index) {
	return func(targetEpoch epoch.Index) {
		eg.Go(func() (err error) {
			if !validPeers.ForEach(func(peerID identity.ID, neighbor *p2p.Neighbor) (success bool) {
				if discardedPeers.Has(peerID) {
					m.log.Debugw("skipping discarded peer", "peer", peerID)
					return true
				}

				db, _ := database.NewMemDB("")
				tangleTree := smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))

				epochChannels := m.startEpochSyncing(targetEpoch)
				epochChannels.RLock()

				m.protocol.RequestEpochBlocks(targetEpoch, ecChain[targetEpoch], peerID)

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
					m.log.Infow("synced epoch", "epoch", params.targetEpoch, "peer", params.neighbor)
				}).WithErrorCallback(func(flowErr error, params *syncingFlowParams) {
					discardedPeers.Add(params.neighbor.ID())
					m.log.Warnf("error while syncing epoch %d from peer %s: %s", params.targetEpoch, params.neighbor, flowErr)
				}).Run(&syncingFlowParams{
					ctx:           errCtx,
					targetEpoch:   targetEpoch,
					targetEC:      ecChain[targetEpoch],
					targetPrevEC:  ecChain[targetEpoch-1],
					epochChannels: epochChannels,
					neighbor:      neighbor,
					tangleTree:    tangleTree,
					epochBlocks:   make(map[models.BlockID]*models.Block),
				})

				return !success
			}) {
				err = errors.Errorf("unable to sync epoch %d", targetEpoch)
			}

			return
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
	ec := new(commitment.Commitment)
	ec.FromBytes(packetEpochRequest.EpochBlocksRequest.GetEC())
	ecID := ec.ID()

	//m.log.Debugw("received epoch blocks request", "peer", nbr.Peer.ID(), "Index", ei, "ID", ec)

	commitment, _ := m.commitmentManager.Commitment(ecID)
	if commitment == nil {
		m.log.Debugw("epoch blocks request rejected: unknown commitment", "peer", nbr.Peer.ID(), "Index", ei, "ID", ec)
		return
	}

	chain := commitment.Chain()
	if chain == nil {
		m.log.Debugw("epoch blocks request rejected: unknown chain", "peer", nbr.Peer.ID(), "Index", ei, "ID", ec)
		return
	}

	blocksCount := chain.BlocksCount(ei)

	// Send epoch starter.
	m.protocol.SendEpochStarter(ei, ecID, blocksCount, nbr.ID())
	m.log.Debugw("sent epoch start", "peer", nbr.Peer.ID(), "Index", ei, "blocksCount", blocksCount)

	if err := chain.StreamEpochBlocks(ei, func(blocks []*models.Block) {
		m.protocol.SendBlocksBatch(ei, ecID, blocks, nbr.ID())
		m.log.Debugw("sent epoch blocks batch", "peer", nbr.ID(), "Index", ei, "blocksLen", len(blocks))
	}, m.blockBatchSize); err != nil {
		m.log.Errorw("epoch blocks request rejected: unable to stream blocks", "peer", nbr.Peer.ID(), "Index", ei, "ID", ec, "err", err)
		return
	}

	// Send epoch terminator.
	// TODO: m.protocol.SendEpochEnd(ei, ec, commitment.Roots(), nbr.ID())
	m.log.Debugw("sent epoch blocks end", "peer", nbr.ID(), "Index", ei, "ID", ecID.Base58())
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

	m.log.Debugw("received epoch blocks start", "peer", nbr.Peer.ID(), "Index", ei, "blocksCount", epochBlocksStart.GetBlocksCount())

	ec := new(commitment.Commitment)
	ec.FromBytes(epochBlocksStart.GetEC())

	epochChannels.startChan <- &epochSyncStart{
		ei:          ei,
		ec:          ec.ID(),
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
	m.log.Debugw("received epoch blocks", "peer", nbr.Peer.ID(), "Index", ei, "blocksLen", len(blocksBytes))

	for _, blockBytes := range blocksBytes {
		block := new(models.Block)
		if _, err := block.FromBytes(blockBytes); err != nil {
			m.log.Errorw("failed to deserialize block", "peer", nbr.Peer.ID(), "err", err)
			return
		}

		ec := new(commitment.Commitment)
		ec.FromBytes(epochBlocksBatch.GetEC())

		select {
		case <-epochChannels.stopChan:
			return
		case epochChannels.blockChan <- &epochSyncBlock{
			ei:    ei,
			ec:    ec.ID(),
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

	m.log.Debugw("received epoch blocks end", "peer", nbr.Peer.ID(), "Index", ei)

	ec := new(commitment.Commitment)
	ec.FromBytes(packetEpochBlocksEnd.EpochBlocksEnd.GetEC())

	epochSyncEnd := &epochSyncEnd{
		ei:    ei,
		ec:    ec.ID(),
		roots: new(commitment.Roots),
	}

	epochSyncEnd.roots.FromBytes(packetEpochBlocksEnd.EpochBlocksEnd.GetRoots())

	epochChannels.endChan <- epochSyncEnd
}

func (m *Manager) getEpochChannels(ei epoch.Index) *epochChannels {
	m.syncingLock.RLock()
	defer m.syncingLock.RUnlock()

	if !m.syncingInProgress {
		return nil
	}

	return m.epochsChannels[ei]
}
