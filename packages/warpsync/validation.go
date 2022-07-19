package warpsync

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/p2p"
	wp "github.com/iotaledger/goshimmer/packages/warpsync/warpsyncproto"
	"github.com/iotaledger/hive.go/generics/event"
	"google.golang.org/protobuf/proto"
)

func (m *Manager) ValidateBackwards(start, end epoch.Index, endPrevEC epoch.EC, endECR epoch.ECR) {
	for i := end - 1; i >= start; i-- {
		m.RequestEpochCommittment(i)
	}
}

func (m *Manager) RequestEpochCommittment(index epoch.Index) {
	committmentReq := &wp.EpochCommittmentRequest{Epoch: int64(index)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	m.send(packet)
}