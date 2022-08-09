package warpsync

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/set"
)

type neighborCommitment struct {
	neighbor *p2p.Neighbor
	ecRecord *epoch.ECRecord
}

func (m *Manager) ValidateBackwards(ctx context.Context, start, end epoch.Index, startEC, endPrevEC epoch.EC) (ecChain map[epoch.Index]epoch.EC, validPeers *set.AdvancedSet[*peer.Peer], err error) {
	if m.IsStopped() {
		return nil, nil, errors.Errorf("warpsync manager is stopped")
	}

	m.startValidation()
	defer m.endValidation()

	ecChain = make(map[epoch.Index]epoch.EC)
	ecRecordChain := make(map[epoch.Index]*epoch.ECRecord)
	validPeers = set.NewAdvancedSet(m.p2pManager.AllNeighborsPeers()...)
	neighborCommitments := make(map[epoch.Index]map[*peer.Peer]*neighborCommitment)

	// We do not request the start nor the ending epoch, as we know the beginning (snapshot) and the end (tip received via gossip) of the chain.
	startRange := start + 1
	endRange := end - 1
	for ei := endRange; ei >= startRange; ei-- {
		m.requestEpochCommittment(ei)
	}

	epochToValidate := endRange
	ecChain[start] = startEC
	ecRecordChain[end] = epoch.NewECRecord(end)
	ecRecordChain[end].SetPrevEC(endPrevEC)

	m.log.Debugw("validation readloop", "start", start, "end", end, "startEC", startEC.Base58(), "endPrevEC", endPrevEC.Base58())

	for {
		select {
		case commitment, ok := <-m.commitmentsChan:
			if !ok {
				return nil, nil, nil
			}
			ecRecord := commitment.ecRecord
			neighborPeer := commitment.neighbor.Peer
			commitmentEI := ecRecord.EI()
			m.log.Debugw("read committment", "EI", commitmentEI, "EC", ecRecord.ComputeEC().Base58())
			// Ignore invalid neighbor.
			if !validPeers.Has(neighborPeer) {
				m.log.Debugw("ignoring invalid neighbor", "ID", neighborPeer)
				continue
			}
			// Ignore committments outside of the range.
			if commitmentEI < startRange || commitmentEI > endRange {
				m.log.Debugw("ignoring committment outside of requested range", "EI", commitmentEI)
				continue
			}
			// If we already validated this epoch, we check if the neighbor is on the target chain.
			if commitmentEI > epochToValidate {
				if ecRecordChain[commitmentEI].ComputeEC() != ecRecord.ComputeEC() {
					validPeers.Delete(neighborPeer)
				}
			}
			// We received a committment out of order, let's save it for later use.
			if commitmentEI < epochToValidate {
				if neighborCommitments[commitmentEI] == nil {
					neighborCommitments[commitmentEI] = make(map[*peer.Peer]*neighborCommitment)
				}
				neighborCommitments[commitmentEI][neighborPeer] = commitment
			}

			// commitmentEI == epochToValidate
			if ecRecordChain[epochToValidate+1].PrevEC() != ecRecord.ComputeEC() {
				m.log.Debugw("neighbor %s sent committment outside of the target chain", neighborPeer)
				validPeers.Delete(neighborPeer)
				continue
			}
			ecRecordChain[epochToValidate] = ecRecord

			// Validate commitments collected so far.
			for {
				neighborCommitmentsForEpoch, received := neighborCommitments[epochToValidate]
				// We haven't received commitments for this epoch yet.
				if !received {
					break
				}

				for neighborID, epochCommitment := range neighborCommitmentsForEpoch {
					proposedECRecord := epochCommitment.ecRecord
					if ecRecordChain[epochToValidate+1].PrevEC() != proposedECRecord.ComputeEC() {
						m.log.Debugw("neighbor %s sent committment outside of the target chain", neighborID)
						validPeers.Delete(neighborID)
						continue
					}

					// We store the valid committment for this chain.
					ecRecordChain[epochToValidate] = proposedECRecord
					ecChain[epochToValidate] = proposedECRecord.ComputeEC()
				}

				// Stop if we were not able to validate epochToValidate.
				if _, exists := ecRecordChain[epochToValidate]; !exists {
					break
				}

				// We validated the epoch and identified the neighbors that are on the target chain.
				epochToValidate--
				m.log.Debugf("epochs left %d", epochToValidate-startRange)
			}

			if epochToValidate == start {
				syncedStartPrevEC := ecRecordChain[start+1].PrevEC()
				if startEC != syncedStartPrevEC {
					return nil, nil, errors.Errorf("obtained chain does not match expected starting point EC: expected %s, actual %s", startEC, syncedStartPrevEC)
				}
				return ecChain, validPeers, nil
			}

		case <-ctx.Done():
			return nil, nil, errors.Errorf("cancelled while validating epoch range %d to %d: %s", start, end, ctx.Err())
		}
	}
}

func (m *Manager) startValidation() {
	m.validationLock.Lock()
	defer m.validationLock.Unlock()
	m.validationInProgress = true
	m.commitmentsChan = make(chan *neighborCommitment)
	m.commitmentsStopChan = make(chan struct{})
}

func (m *Manager) endValidation() {
	close(m.commitmentsStopChan)
	m.validationLock.Lock()
	defer m.validationLock.Unlock()
	m.validationInProgress = false
	close(m.commitmentsChan)
}

func (m *Manager) processEpochCommittmentRequestPacket(packetEpochRequest *wp.Packet_EpochCommitmentRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochCommitmentRequest.GetEI())
	m.log.Debugw("received epoch committment request", "peer", nbr.Peer.ID(), "EI", ei)

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists {
		return
	}

	m.sendEpochCommittmentMessage(ei, ecRecord.ECR(), ecRecord.PrevEC(), nbr.ID())

	m.log.Debugw("sent epoch committment", "peer", nbr.Peer.ID(), "EI", ei, "EC", ecRecord.ComputeEC().Base58())
}

func (m *Manager) processEpochCommittmentPacket(packetEpochCommittment *wp.Packet_EpochCommitment, nbr *p2p.Neighbor) {
	m.validationLock.RLock()
	defer m.validationLock.RUnlock()

	if !m.validationInProgress {
		return
	}

	ei := epoch.Index(packetEpochCommittment.EpochCommitment.GetEI())
	ecr := epoch.NewMerkleRoot(packetEpochCommittment.EpochCommitment.GetECR())
	prevEC := epoch.NewMerkleRoot(packetEpochCommittment.EpochCommitment.GetPrevEC())

	ecRecord := epoch.NewECRecord(ei)
	ecRecord.SetECR(ecr)
	ecRecord.SetPrevEC(prevEC)

	m.log.Debugw("received epoch committment", "peer", nbr.Peer.ID(), "EI", ei, "EC", ecRecord.ComputeEC().Base58())

	select {
	case <-m.commitmentsStopChan:
		return
	case m.commitmentsChan <- &neighborCommitment{
		neighbor: nbr,
		ecRecord: ecRecord,
	}:
	}
}
