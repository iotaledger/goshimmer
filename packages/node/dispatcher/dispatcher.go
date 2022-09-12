package dispatcher

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Dispatcher struct {
	Events *Events

	forkManager *ForkManager
	protocols   map[epoch.EC]*protocol.Protocol
}

func New() (dispatcher *Dispatcher) {
	return &Dispatcher{
		Events:    NewEvents(),
		protocols: make(map[epoch.EC]*protocol.Protocol),
	}
}

func (p *Dispatcher) DispatchBlockData(bytes []byte, neighbor *p2p.Neighbor) {
	block := new(models.Block)
	if _, err := block.FromBytes(bytes); err != nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	protocol := p.protocol(block)

	return
}

func (p *Dispatcher) protocol(block *models.Block) (protocol *protocol.Protocol) {
	if fork := p.forkManager.Fork(epoch.NewEpochCommitment(block.EI(), block.ECR(), block.PrevEC())); fork != nil {
		return p.protocols[fork.ID()]
	}

	protocol = p.protocols[fork.ID()]

	block.EI()
	block.PrevEC()
	block.ECR()

	if protocol == nil {
		protocol = protocol.New()
		p.protocols[block.Protocol] = protocol
	}
	return
}

func (p *Dispatcher) ChainID(ecr epoch.ECR) string {

}
