package warpsync

import (
	"context"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/identity"
)

type neighborCommitment struct {
	neighbor *p2p.Neighbor
	ecRecord *epoch.ECRecord
}

func (m *Manager) ValidateBackwards(ctx context.Context, start, end epoch.Index, startEC, endPrevEC epoch.EC) (ecChain map[epoch.Index]*epoch.ECRecord, validNeighbors *set.AdvancedSet[identity.ID], err error) {
	if m.IsStopped() {
		return ecChain, validNeighbors, fmt.Errorf("warpsync manager is stopped")
	}

	m.startValidation()
	defer m.endValidation()

	ecChain = make(map[epoch.Index]*epoch.ECRecord)
	validNeighbors = set.NewAdvancedSet(m.p2pManager.AllNeighborsIDs()...)
	neighborCommitments := make(map[epoch.Index]map[identity.ID]*neighborCommitment)

	// We do not request the start nor the ending epoch, as we know the beginning (snapshot) and the end (tip received via gossip) of the chain.
	startRange := start + 1
	endRange := end - 1
	for ei := endRange; ei >= startRange; ei-- {
		m.requestEpochCommittment(ei)
	}

	epochToValidate := endRange
	ecChain[end] = epoch.NewECRecord(end)
	ecChain[end].SetPrevEC(endPrevEC)

	m.log.Debugw("validation readloop", "start", start, "end", end, "startEC", startEC.Base58(), "endPrevEC", endPrevEC.Base58())

	for {
		select {
		case commitment := <-m.commitmentsChan:
			ecRecord := commitment.ecRecord
			neighborID := commitment.neighbor.ID()
			commitmentEI := ecRecord.EI()
			m.log.Debugw("read committment", "EI", commitmentEI, "EC", ecRecord.ComputeEC().Base58())
			// Ignore invalid neighbor.
			if !validNeighbors.Has(neighborID) {
				m.log.Debugw("ignoring invalid neighbor", "ID", neighborID)
				continue
			}
			// Ignore committments outside of the range.
			if commitmentEI < startRange || commitmentEI > endRange {
				m.log.Debugw("ignoring committment outside of requested range", "EI", commitmentEI)
				continue
			}
			// If we already validated this epoch, we check if the neighbor is on the target chain.
			if commitmentEI > epochToValidate {
				if ecChain[commitmentEI].ComputeEC() != ecRecord.ComputeEC() {
					validNeighbors.Delete(neighborID)
				}
			}
			// We received a committment out of order, let's save it for later use.
			if commitmentEI < epochToValidate {
				if neighborCommitments[commitmentEI] == nil {
					neighborCommitments[commitmentEI] = make(map[identity.ID]*neighborCommitment)
				}
				neighborCommitments[commitmentEI][neighborID] = commitment
			}

			// commitmentEI == epochToValidate
			if ecChain[epochToValidate+1].PrevEC() != ecRecord.ComputeEC() {
				m.log.Debugw("neighbor %s sent committment outside of the target chain", neighborID)
				validNeighbors.Delete(neighborID)
				continue
			}
			ecChain[epochToValidate] = ecRecord

			// Validate commitments collected so far.
			for {
				neighborCommitmentsForEpoch, received := neighborCommitments[epochToValidate]
				// We haven't received commitments for this epoch yet.
				if !received {
					break
				}

				for neighborID, epochCommitment := range neighborCommitmentsForEpoch {
					proposedECRecord := epochCommitment.ecRecord
					if ecChain[epochToValidate+1].PrevEC() != proposedECRecord.ComputeEC() {
						m.log.Debugw("neighbor %s sent committment outside of the target chain", neighborID)
						validNeighbors.Delete(neighborID)
						continue
					}

					// We store the valid committment for this chain.
					ecChain[epochToValidate] = proposedECRecord
				}

				// Stop if we were not able to validate epochToValidate.
				if _, exists := ecChain[epochToValidate]; !exists {
					break
				}

				// We validated the epoch and identified the neighbors that are on the target chain.
				epochToValidate--
				m.log.Debugf("epochs left %d", epochToValidate-startRange)
			}

			if epochToValidate == startRange {
				syncedStartEC := ecChain[startRange].PrevEC()
				if startEC != syncedStartEC {
					return ecChain, validNeighbors, fmt.Errorf("obtained chain does not match expected starting point EC: expected %s, actual %s", startEC, syncedStartEC)
				}
				return ecChain, validNeighbors, nil
			}

		case <-ctx.Done():
			return ecChain, validNeighbors, fmt.Errorf("cancelled while validating epoch range %d to %d: %s", start, end, ctx.Err())
		}
	}
}

func (m *Manager) startValidation() {
	m.validationLock.Lock()
	defer m.validationLock.Unlock()
	m.validationInProgress = true
	m.commitmentsStopChan = make(chan struct{})
	m.commitmentsChan = make(chan *neighborCommitment)
}

func (m *Manager) endValidation() {
	close(m.commitmentsStopChan)
	m.validationLock.Lock()
	defer m.validationLock.Unlock()
	m.validationInProgress = false
	close(m.commitmentsChan)
}

func (m *Manager) requestEpochCommittment(ei epoch.Index) {
	committmentReq := &wp.EpochCommittmentRequest{EI: int64(ei)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	m.p2pManager.Send(packet, protocolID)
	m.log.Debugw("sent epoch committment request", "EI", ei)
}

func (m *Manager) processEpochCommittmentRequestPacket(packetEpochRequest *wp.Packet_EpochCommitmentRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochCommitmentRequest.GetEI())
	m.log.Debugw("received epoch committment request", "peer", nbr.Peer.ID(), "EI", ei)

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists {
		return
	}

	committmentRes := &wp.EpochCommittment{
		EI:     int64(ei),
		ECR:    ecRecord.ECR().Bytes(),
		PrevEC: ecRecord.PrevEC().Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitment{EpochCommitment: committmentRes}}

	m.p2pManager.Send(packet, protocolID, nbr.ID())

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
