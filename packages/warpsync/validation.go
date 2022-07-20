package warpsync

import (
	"context"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/warpsync/warpsyncproto"
)

func (m *Manager) ValidateBackwards(ctx context.Context, start, end epoch.Index, startEC, endPrevEC epoch.EC) (bool, error) {
	if m.validationInProgress {
		return false, fmt.Errorf("epoch validation already in progress")
	}

	m.validationInProgress = true
	defer func() { m.validationInProgress = false }()

	for ei := end - 1; ei >= start; ei-- {
		m.RequestEpochCommittment(ei)
	}

	ecRecords := make(map[epoch.Index]*epoch.ECRecord)
	toReceive := end - start

readLoop:
	for {
		select {
		case ecRecord := <-m.commitmentsChan:
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

func (m *Manager) RequestEpochCommittment(index epoch.Index) {
	committmentReq := &wp.EpochCommittmentRequest{EI: int64(index)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	m.send(packet)
}

func (m *Manager) processEpochCommittmentRequestPacket(packetEpochRequest *wp.Packet_EpochCommitmentRequest, nbr *p2p.Neighbor) {
	// TOOD
	/*
		var blkID tangle.BlockID
		_, err := blkID.Decode(packetEpochBlocks.BlockRequest.GetId())
		if err != nil {
			m.log.Debugw("invalid block id:", "err", err)
			return
		}

		blkBytes, err := m.loadBlockFunc(blkID)
		if err != nil {
			m.log.Debugw("error loading block", "blk-id", blkID, "err", err)
			return
		}

		// send the loaded block directly to the neighbor
		packet := &gp.Packet{Body: &gp.Packet_Block{Block: &gp.Block{Data: blkBytes}}}
		if err := nbr.GetStream(protocolID).WritePacket(packet); err != nil {
			nbr.Log.Warnw("Failed to send requested block back to the neighbor", "err", err)
			nbr.Close()
		}
		m.Events.BlockReceived.Trigger(&BlockReceivedEvent{Data: packetEpochRequest.Block.GetData(), Peer: nbr.Peer})
	*/
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
