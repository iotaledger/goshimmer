package warpsync

import (
	"context"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
)

func (m *Manager) ValidateBackwards(ctx context.Context, start, end epoch.Index, startEC, endPrevEC epoch.EC) (ecChain map[epoch.Index]*epoch.ECRecord, err error) {

	if m.IsStopped() {
		return nil, fmt.Errorf("warpsync manager is stopped")
	}

	m.validationInProgress = true
	defer func() { m.validationInProgress = false }()

	ecChain = make(map[epoch.Index]*epoch.ECRecord)

	// We do not request the start nor the ending epoch, as we know the beginning (snapshot) and the end (tip received via gossip) of the chain.
	for ei := end - 1; ei > start; ei-- {
		m.requestEpochCommittment(ei)
	}
	toReceive := end - start - 1

	m.log.Debugw("validation readloop", "start", start, "end", end, "startEC", startEC.Base58(), "endPrevEC", endPrevEC.Base58())

readLoop:
	for {
		select {
		case ecRecord := <-m.commitmentsChan:
			m.log.Debugw("read committment", "EI", ecRecord.EI(), "EC", ecRecord.ComputeEC().Base58())
			if ecChain[ecRecord.EI()] != nil {
				continue
			}
			ecChain[ecRecord.EI()] = ecRecord
			toReceive--

			m.log.Debugf("epochs left %d", toReceive)
			if toReceive == 0 {
				break readLoop
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("cancelled while validating epoch range %d to %d: %s", start, end, ctx.Err())
		}
	}

	ecChain[end] = epoch.NewECRecord(end)
	ecChain[end].SetPrevEC(endPrevEC)

	for ei := end - 1; ei > start; ei-- {
		ecRecord, exists := ecChain[ei]
		if !exists {
			return nil, fmt.Errorf("did not receive epoch commitment for epoch %d", ei)
		}
		if ecChain[ei+1].PrevEC() != ecRecord.ComputeEC() {
			return nil, fmt.Errorf("epoch EC of epoch %d does not match PrevEC of epoch %d", ecRecord.EI(), ei+1)
		}
	}

	actualEC := ecChain[start+1].PrevEC()
	if startEC != actualEC {
		return nil, fmt.Errorf("obtained chain does not match expected starting point EC: expected %s, actual %s", startEC, actualEC)
	}

	m.log.Debugw("validation successful")
	return ecChain, nil
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

	m.commitmentsChan <- ecRecord
}
