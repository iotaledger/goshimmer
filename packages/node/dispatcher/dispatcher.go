package dispatcher

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/epoch/commitmentmanager"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Dispatcher struct {
	Events *Events

	activeProtocol    *protocol.Protocol
	protocolsByChain  map[epoch.EC]*protocol.Protocol
	commitmentManager commitmentmanager.CommitmentManager
}

func New() (dispatcher *Dispatcher) {
	return &Dispatcher{
		Events:           NewEvents(),
		protocolsByChain: make(map[epoch.EC]*protocol.Protocol),
	}
}

func (p *Dispatcher) DispatchBlockData(bytes []byte, neighbor *p2p.Neighbor) {
	block := new(models.Block)
	if _, err := block.FromBytes(bytes); err != nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	return
}
