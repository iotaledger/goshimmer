package warpsync

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/network/warpsync"
)

type neighborCommitment struct {
	neighbor *p2p.Neighbor
	ecRecord *epoch.ECRecord
}

func (m *Manager) validateBackwards(ctx context.Context, start, end epoch.Index, startEC, endPrevEC epoch.EC) (ecChain map[epoch.Index]epoch.EC, activePeers *set.AdvancedSet[identity.ID], err error) {
	m.startValidation()
	defer m.endValidation()

	ecChain = make(map[epoch.Index]epoch.EC)
	ecRecordChain := make(map[epoch.Index]*epoch.ECRecord)
	activePeers = set.NewAdvancedSet[identity.ID]()
	discardedPeers := set.NewAdvancedSet[identity.ID]()
	neighborCommitments := make(map[epoch.Index]map[identity.ID]*neighborCommitment)

	// We do not request the start nor the ending epoch, as we know the beginning (snapshot) and the end (tip received via gossip) of the chain.
	startRange := start + 1
	endRange := end - 1
	for ei := endRange; ei >= startRange; ei-- {
		m.protocol.RequestEpochCommittment(ei)
	}

	epochToValidate := endRange
	ecChain[start] = startEC
	ecRecordChain[end] = epoch.NewECRecord(end)
	ecRecordChain[end].SetPrevEC(endPrevEC)

	for {
		select {
		case commitment, ok := <-m.commitmentsChan:
			if !ok {
				return nil, nil, nil
			}
			ecRecord := commitment.ecRecord
			peerID := commitment.neighbor.Peer.ID()
			commitmentEI := ecRecord.EI()
			m.log.Debugw("read committment", "EI", commitmentEI, "EC", ecRecord.ComputeEC().Base58())

			activePeers.Add(peerID)
			// Ignore invalid neighbor.
			if discardedPeers.Has(peerID) {
				m.log.Debugw("ignoring invalid neighbor", "ID", peerID, "validPeers", activePeers, "discardedPeers", discardedPeers)
				continue
			}

			// Ignore committments outside of the range.
			if commitmentEI < startRange || commitmentEI > endRange {
				m.log.Debugw("ignoring committment outside of requested range", "EI", commitmentEI, "peer", peerID)
				continue

			}

			// If we already validated this epoch, we check if the neighbor is on the target chain.
			if commitmentEI > epochToValidate {
				if ecRecordChain[commitmentEI].ComputeEC() != ecRecord.ComputeEC() {
					m.log.Infow("ignoring commitment outside of the target chain", "peer", peerID)
					discardedPeers.Add(peerID)
				}
				continue
			}

			// commitmentEI <= epochToValidate
			if neighborCommitments[commitmentEI] == nil {
				neighborCommitments[commitmentEI] = make(map[identity.ID]*neighborCommitment)
			}
			neighborCommitments[commitmentEI][peerID] = commitment

			// We received a committment out of order, we can evaluate it only later.
			if commitmentEI < epochToValidate {
				continue
			}

			// commitmentEI == epochToValidate
			// Validate commitments collected so far.
			for {
				neighborCommitmentsForEpoch, received := neighborCommitments[epochToValidate]
				// We haven't received commitments for this epoch yet.
				if !received {
					break
				}

				for peerID, epochCommitment := range neighborCommitmentsForEpoch {
					if discardedPeers.Has(peerID) {
						continue
					}
					proposedECRecord := epochCommitment.ecRecord
					if ecRecordChain[epochToValidate+1].PrevEC() != proposedECRecord.ComputeEC() {
						m.log.Infow("ignoring commitment outside of the target chain", "peer", peerID)
						discardedPeers.Add(peerID)
						continue
					}

					// If we already stored the target epoch for the chain, we just keep validating neighbors.
					if _, exists := ecRecordChain[epochToValidate]; exists {
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
				m.log.Debugf("epochs left %d", epochToValidate-start)
			}

			if epochToValidate == start {
				syncedStartPrevEC := ecRecordChain[start+1].PrevEC()
				if startEC != syncedStartPrevEC {
					return nil, nil, errors.Errorf("obtained chain does not match expected starting point EC: expected %s, actual %s", startEC.Base58(), syncedStartPrevEC.Base58())
				}
				m.log.Infof("range %d-%d validated", start, end)
				activePeers.DeleteAll(discardedPeers)
				return ecChain, activePeers, nil
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

func (m *Manager) onEpochCommittmentReceived(event *warpsync.EpochCommittmentReceivedEvent) {
	m.validationLock.RLock()
	defer m.validationLock.RUnlock()

	if !m.validationInProgress {
		return
	}

	select {
	case <-m.commitmentsStopChan:
		return
	case m.commitmentsChan <- &neighborCommitment{
		neighbor: event.Neighbor,
		ecRecord: event.ECRecord,
	}:
	}
}
