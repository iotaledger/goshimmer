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
	if m.validationInProgress {
		return nil, fmt.Errorf("epoch validation already in progress")
	}

	if m.isStopped {
		return nil, fmt.Errorf("warpsync manager is stopped")
	}

	m.validationInProgress = true
	defer func() { m.validationInProgress = false }()

	for ei := end - 1; ei >= start; ei-- {
		m.requestEpochCommittment(ei)
	}

	ecChain = make(map[epoch.Index]*epoch.ECRecord)
	toReceive := end - start

readLoop:
	for {
		select {
		case ecRecord := <-m.commitmentsChan:
			if ecChain[ecRecord.EI()] != nil {
				continue
			}
			ecChain[ecRecord.EI()] = ecRecord
			toReceive--
			if toReceive == 0 {
				break readLoop
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("cancelled while validating epoch range %d to %d", start, end)
		}
	}

	ecChain[end] = epoch.NewECRecord(end)
	ecChain[end].SetPrevEC(endPrevEC)

	for ei := end - 1; ei >= start; ei-- {
		ecRecord, exists := ecChain[ei]
		if !exists {
			return nil, fmt.Errorf("did not receive epoch commitment for epoch %d", ei)
		}
		if ecChain[ei+1].PrevEC() != epoch.ComputeEC(ecRecord) {
			return nil, fmt.Errorf("epoch EC of epoch %d does not match PrevEC of epoch %d", ecRecord.EI(), ei+1)
		}
	}

	actualEC := epoch.ComputeEC(ecChain[start])
	if startEC != actualEC {
		return nil, fmt.Errorf("obtained chain does not match expected starting point EC: expected %s, actual %s", startEC, actualEC)
	}

	return ecChain, nil
}

func (m *Manager) requestEpochCommittment(index epoch.Index) {
	committmentReq := &wp.EpochCommittmentRequest{EI: int64(index)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	m.p2pManager.Send(packet, protocolID)
}

func (m *Manager) processEpochCommittmentRequestPacket(packetEpochRequest *wp.Packet_EpochCommitmentRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochCommitmentRequest.GetEI())
	ec := epoch.NewMerkleRoot(packetEpochRequest.EpochCommitmentRequest.GetEC())

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists || ec != epoch.ComputeEC(ecRecord) {
		return
	}

	committmentRes := &wp.EpochCommittment{
		EI:     int64(ei),
		ECR:    ecRecord.ECR().Bytes(),
		PrevEC: ecRecord.PrevEC().Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitment{EpochCommitment: committmentRes}}

	m.p2pManager.Send(packet, protocolID, nbr.ID())
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

	m.commitmentsChan <- ecRecord
}
