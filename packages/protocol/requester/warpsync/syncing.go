package warpsync

import (
	"context"

	"github.com/celestiaorg/smt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/sync/errgroup"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/core/dataflow"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/orderedmap"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
)

type slotSyncStart struct {
	si          slot.Index
	sc          commitment.ID
	blocksCount int64
}

type slotSyncBlock struct {
	si    slot.Index
	sc    commitment.ID
	block *models.Block
	peer  *peer.Peer
}

type slotSyncEnd struct {
	si    slot.Index
	sc    commitment.ID
	roots *commitment.Roots
}

type blockReceived struct {
	block *models.Block
	peer  *peer.Peer
}

func (m *Manager) syncRange(ctx context.Context, start, end slot.Index, startSC commitment.ID, ecChain map[slot.Index]commitment.ID, validPeers *orderedmap.OrderedMap[identity.ID, *p2p.Neighbor]) (completedSlot slot.Index, err error) {
	startRange := start + 1
	endRange := end - 1

	m.startSyncing(startRange, endRange)
	defer m.endSyncing()

	eg, errCtx := errgroup.WithContext(ctx)
	eg.SetLimit(m.concurrency)

	slotProcessedChan := make(chan slot.Index)
	discardedPeers := advancedset.New[identity.ID]()

	workerFunc := m.syncSlotFunc(errCtx, eg, validPeers, discardedPeers, ecChain, slotProcessedChan)
	completedSlot = m.queueSlidingSlots(errCtx, startRange, endRange, workerFunc, slotProcessedChan)

	if err := eg.Wait(); err != nil {
		return completedSlot, errors.Wrapf(err, "sync failed for range %d-%d", start, end)
	}
	return completedSlot, nil
}

func (m *Manager) syncSlotFunc(errCtx context.Context, eg *errgroup.Group, validPeers *orderedmap.OrderedMap[identity.ID, *p2p.Neighbor], discardedPeers *advancedset.AdvancedSet[identity.ID], ecChain map[slot.Index]commitment.ID, slotProcessedChan chan slot.Index) func(targetSlot slot.Index) {
	return func(targetSlot slot.Index) {
		eg.Go(func() (err error) {
			if !validPeers.ForEach(func(peerID identity.ID, neighbor *p2p.Neighbor) (success bool) {
				if discardedPeers.Has(peerID) {
					m.log.Debugw("skipping discarded peer", "peer", peerID)
					return true
				}

				db, _ := database.NewMemDB("")
				tangleTree := smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))

				slotChannels := m.startSlotSyncing(targetSlot)
				slotChannels.RLock()

				m.protocol.RequestSlotBlocks(targetSlot, ecChain[targetSlot], peerID)

				err = dataflow.New(
					m.slotStartCommand,
					m.slotBlockCommand,
					m.slotEndCommand,
					m.slotVerifyCommand,
					m.slotProcessBlocksCommand,
				).WithTerminationCallback(func(params *syncingFlowParams) {
					params.slotChannels.RUnlock()
					m.endSlotSyncing(params.targetSlot)
				}).WithSuccessCallback(func(params *syncingFlowParams) {
					success = true
					select {
					case <-params.ctx.Done():
						return
					case slotProcessedChan <- params.targetSlot:
					}
					m.log.Infow("synced slot", "slot", params.targetSlot, "peer", params.neighbor)
				}).WithErrorCallback(func(flowErr error, params *syncingFlowParams) {
					discardedPeers.Add(params.neighbor.ID())
					m.log.Warnf("error while syncing slot %d from peer %s: %s", params.targetSlot, params.neighbor, flowErr)
				}).Run(&syncingFlowParams{
					ctx:          errCtx,
					targetSlot:   targetSlot,
					targetEC:     ecChain[targetSlot],
					targetPrevEC: ecChain[targetSlot-1],
					slotChannels: slotChannels,
					neighbor:     neighbor,
					tangleTree:   tangleTree,
					slotBlocks:   make(map[models.BlockID]*models.Block),
				})

				return !success
			}) {
				err = errors.Errorf("unable to sync slot %d", targetSlot)
			}

			return
		})
	}
}

func (m *Manager) queueSlidingSlots(errCtx context.Context, startRange, endRange slot.Index, workerFunc func(slot.Index), slotProcessedChan chan slot.Index) (completedSlot slot.Index) {
	processedSlots := make(map[slot.Index]types.Empty)
	for ei := startRange; ei < startRange+slot.Index(m.concurrency) && ei <= endRange; ei++ {
		workerFunc(ei)
	}

	windowStart := startRange
	for {
		select {
		case processedSlot := <-slotProcessedChan:
			processedSlots[processedSlot] = types.Void
			for {
				if _, processed := processedSlots[windowStart]; processed {
					completedSlot = windowStart
					if completedSlot == endRange {
						return
					}
					windowEnd := windowStart + slot.Index(m.concurrency)
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

func (m *Manager) startSyncing(startRange, endRange slot.Index) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	m.syncingInProgress = true
	m.slotsChannels = make(map[slot.Index]*slotChannels)
	for ei := startRange; ei <= endRange; ei++ {
		m.slotsChannels[ei] = &slotChannels{}
	}
}

func (m *Manager) endSyncing() {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	m.syncingInProgress = false
	m.slotsChannels = nil
}

func (m *Manager) startSlotSyncing(ei slot.Index) (slotChannels *slotChannels) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	slotChannels = m.slotsChannels[ei]

	slotChannels.Lock()
	defer slotChannels.Unlock()

	slotChannels.startChan = make(chan *slotSyncStart, 1)
	slotChannels.blockChan = make(chan *slotSyncBlock, 1)
	slotChannels.endChan = make(chan *slotSyncEnd, 1)
	slotChannels.stopChan = make(chan struct{})
	slotChannels.active = true

	return
}

func (m *Manager) endSlotSyncing(ei slot.Index) {
	m.syncingLock.Lock()
	defer m.syncingLock.Unlock()

	slotChannels := m.slotsChannels[ei]

	slotChannels.active = false
	close(slotChannels.stopChan)
	slotChannels.Lock()
	defer slotChannels.Unlock()

	close(slotChannels.startChan)
	close(slotChannels.blockChan)
	close(slotChannels.endChan)
}

func (m *Manager) processSlotBlocksRequestPacket(packetSlotRequest *wp.Packet_SlotBlocksRequest, nbr *p2p.Neighbor) {
	si := slot.Index(packetSlotRequest.SlotBlocksRequest.GetSI())
	sc := new(commitment.Commitment)
	if _, err := sc.FromBytes(packetSlotRequest.SlotBlocksRequest.GetSC()); err != nil {
		m.log.Errorw("slot blocks request rejected: unable to deserialize commitment", "err", err)
		return
	}
	scID := sc.ID()

	// m.log.Debugw("received slot blocks request", "peer", nbr.Peer.ID(), "Index", ei, "ID", ec)

	// cm, _ := m.commitmentManager.commitment(scID)
	// if cm == nil {
	// 	m.log.Debugw("slot blocks request rejected: unknown commitment", "peer", nbr.Peer.ID(), "Index", si, "ID", sc)
	// 	return
	// }

	// chain := cm.Chain()
	// if chain == nil {
	// 	m.log.Debugw("slot blocks request rejected: unknown chain", "peer", nbr.Peer.ID(), "Index", si, "ID", sc)
	// 	return
	// }

	// blocksCount := chain.BlocksCount(ei)

	//// Send slot starter.
	//m.protocol.SendSlotStarter(ei, ecID, blocksCount, nbr.ID())
	//m.log.Debugw("sent slot start", "peer", nbr.Peer.ID(), "Index", ei, "blocksCount", blocksCount)
	//
	//if err := chain.StreamSlotBlocks(ei, func(blocks []*models.Block) {
	//	m.protocol.SendBlocksBatch(ei, ecID, blocks, nbr.ID())
	//	m.log.Debugw("sent slot blocks batch", "peer", nbr.ID(), "Index", ei, "blocksLen", len(blocks))
	//}, m.blockBatchSize); err != nil {
	//	m.log.Errorw("slot blocks request rejected: unable to stream blocks", "peer", nbr.Peer.ID(), "Index", ei, "ID", ec, "err", err)
	//	return
	//}

	// Send slot terminator.
	// TODO: m.protocol.SendSlotEnd(ei, ec, commitment.Roots(), nbr.ID())
	m.log.Debugw("sent slot blocks end", "peer", nbr.ID(), "Index", si, "ID", scID.Base58())
}

func (m *Manager) processSlotBlocksStartPacket(packetSlotBlocksStart *wp.Packet_SlotBlocksStart, nbr *p2p.Neighbor) {
	slotBlocksStart := packetSlotBlocksStart.SlotBlocksStart
	si := slot.Index(slotBlocksStart.GetSI())

	slotChannels := m.getSlotChannels(si)
	if slotChannels == nil {
		return
	}

	slotChannels.RLock()
	defer slotChannels.RUnlock()

	if !slotChannels.active {
		return
	}

	m.log.Debugw("received slot blocks start", "peer", nbr.Peer.ID(), "Index", si, "blocksCount", slotBlocksStart.GetBlocksCount())

	sc := new(commitment.Commitment)
	if _, err := sc.FromBytes(slotBlocksStart.GetSC()); err != nil {
		m.log.Errorw("received slot blocks start: unable to deserialize commitment", "err", err)
		return
	}

	slotChannels.startChan <- &slotSyncStart{
		si:          si,
		sc:          sc.ID(),
		blocksCount: slotBlocksStart.GetBlocksCount(),
	}
}

func (m *Manager) processSlotBlocksBatchPacket(packetSlotBlocksBatch *wp.Packet_SlotBlocksBatch, nbr *p2p.Neighbor) {
	slotBlocksBatch := packetSlotBlocksBatch.SlotBlocksBatch
	si := slot.Index(slotBlocksBatch.GetSI())

	slotChannels := m.getSlotChannels(si)
	if slotChannels == nil {
		return
	}

	slotChannels.RLock()
	defer slotChannels.RUnlock()

	if !slotChannels.active {
		return
	}

	blocksBytes := slotBlocksBatch.GetBlocks()
	m.log.Debugw("received slot blocks", "peer", nbr.Peer.ID(), "Index", si, "blocksLen", len(blocksBytes))

	for _, blockBytes := range blocksBytes {
		block := new(models.Block)
		if _, err := block.FromBytes(blockBytes); err != nil {
			m.log.Errorw("failed to deserialize block", "peer", nbr.Peer.ID(), "err", err)
			return
		}

		sc := new(commitment.Commitment)
		if _, err := sc.FromBytes(slotBlocksBatch.GetSC()); err != nil {
			m.log.Errorw("failed to deserialize commitment", "peer", nbr.Peer.ID(), "err", err)
			return
		}

		select {
		case <-slotChannels.stopChan:
			return
		case slotChannels.blockChan <- &slotSyncBlock{
			si:    si,
			sc:    sc.ID(),
			peer:  nbr.Peer,
			block: block,
		}:
		}
	}
}

func (m *Manager) processSlotBlocksEndPacket(packetSlotBlocksEnd *wp.Packet_SlotBlocksEnd, nbr *p2p.Neighbor) {
	slotBlocksBatch := packetSlotBlocksEnd.SlotBlocksEnd
	si := slot.Index(slotBlocksBatch.GetSI())

	slotChannels := m.getSlotChannels(si)
	if slotChannels == nil {
		return
	}

	slotChannels.RLock()
	defer slotChannels.RUnlock()

	if !slotChannels.active {
		return
	}

	m.log.Debugw("received slot blocks end", "peer", nbr.Peer.ID(), "Index", si)

	sc := new(commitment.Commitment)
	if _, err := sc.FromBytes(packetSlotBlocksEnd.SlotBlocksEnd.GetSC()); err != nil {
		m.log.Errorw("received slot blocks end: unable to deserialize commitment", "err", err)
		return
	}

	slotSyncEnd := &slotSyncEnd{
		si:    si,
		sc:    sc.ID(),
		roots: new(commitment.Roots),
	}

	if _, err := slotSyncEnd.roots.FromBytes(packetSlotBlocksEnd.SlotBlocksEnd.GetRoots()); err != nil {
		m.log.Errorw("received slot blocks end: unable to deserialize roots", "err", err)
		return
	}

	slotChannels.endChan <- slotSyncEnd
}

func (m *Manager) getSlotChannels(ei slot.Index) *slotChannels {
	m.syncingLock.RLock()
	defer m.syncingLock.RUnlock()

	if !m.syncingInProgress {
		return nil
	}

	return m.slotsChannels[ei]
}
