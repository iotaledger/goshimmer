package warpsync

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
)

func (p *Protocol) RequestEpochCommittment(ei epoch.Index, to ...identity.ID) {
	committmentReq := &wp.EpochCommittmentRequest{EI: int64(ei)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	p.p2pManager.Send(packet, protocolID, to...)
	p.log.Debugw("sent epoch committment request", "Index", ei)
}
