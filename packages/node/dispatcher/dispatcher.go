package dispatcher

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/commitment/chain"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Dispatcher struct {
	Events *Events

	activeProtocol    *protocol.Protocol
	protocolsByChain  map[commitment.ID]*protocol.Protocol
	commitmentManager *chain.Manager
}

func New(snapshotIndex epoch.Index, snapshotRootsID commitment.RootsID, snapshotPrevID commitment.ID) (dispatcher *Dispatcher) {
	return &Dispatcher{
		Events:            NewEvents(),
		protocolsByChain:  make(map[commitment.ID]*protocol.Protocol),
		commitmentManager: chain.NewManager(snapshotIndex, snapshotRootsID, snapshotPrevID),
	}
}

func (p *Dispatcher) DispatchBlockData(bytes []byte, neighbor *p2p.Neighbor) {
	block := new(models.Block)
	if _, err := block.FromBytes(bytes); err != nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	p.commitmentManager.ProcessCommitment(block.EI(), block.ECR(), block.PrevEC())

	return
}
