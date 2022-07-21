package warpsync

import (
	"context"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
)

func (m *Manager) ValidateBackwards(ctx context.Context, start, end epoch.Index, startEC, endPrevEC epoch.EC) (bool, error) {
	if m.validationInProgress {
		return false, fmt.Errorf("epoch validation already in progress")
	}

	m.validationInProgress = true
	defer func() { m.validationInProgress = false }()

	for ei := end - 1; ei >= start; ei-- {
		m.requestEpochCommittment(ei)
	}

	ecRecords := make(map[epoch.Index]*epoch.ECRecord)
	toReceive := end - start

readLoop:
	for {
		select {
		case ecRecord := <-m.commitmentsChan:
			if ecRecords[ecRecord.EI()] != nil {
				continue
			}
			ecRecords[ecRecord.EI()] = ecRecord
			toReceive--
			if toReceive == 0 {
				break readLoop
			}
		case <-ctx.Done():
			return false, fmt.Errorf("cancelled while validating epoch range %d to %d", start, end)
		}
	}

	ecRecords[end] = epoch.NewECRecord(end)
	ecRecords[end].SetPrevEC(endPrevEC)

	for ei := end - 1; ei >= start; ei-- {
		ecRecord, exists := ecRecords[ei]
		if !exists {
			return false, fmt.Errorf("did not receive epoch commitment for epoch %d", ei)
		}
		if ecRecords[ei+1].PrevEC() != notarization.EC(ecRecord) {
			return false, fmt.Errorf("epoch EC of epoch %d does not match PrevEC of epoch %d", ecRecord.EI(), ei+1)
		}
	}

	return startEC == notarization.EC(ecRecords[start]), nil
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
	if !exists || ec != notarization.EC(ecRecord) {
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
