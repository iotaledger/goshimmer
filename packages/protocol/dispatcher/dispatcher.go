package dispatcher

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment/chainmanager"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/models"
	chainmanager2 "github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
)

type Dispatcher struct {
	Events *Events

	activeProtocol   *chainmanager.Protocol
	protocolsByChain map[commitment.ID]*chainmanager.Protocol
	chainManager     *chainmanager2.Manager
}

func New(snapshotIndex epoch.Index, snapshotRootsID commitment.RootsID, snapshotPrevID commitment.ID) (dispatcher *Dispatcher) {
	return &Dispatcher{
		Events:           NewEvents(),
		protocolsByChain: make(map[commitment.ID]*chainmanager.Protocol),
		chainManager:     chainmanager2.NewManager(snapshotIndex, snapshotRootsID, snapshotPrevID),
	}
}

func (p *Dispatcher) DispatchBlockData(bytes []byte, neighbor *p2p.Neighbor) {
	block := new(models.Block)
	if _, err := block.FromBytes(bytes); err != nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	p.chainManager.ProcessCommitment(block.EI(), block.ECR(), block.PrevEC())

	return
}
