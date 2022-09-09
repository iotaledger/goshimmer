package dispatcher

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Dispatcher struct {
	Events *Events

	protocols map[string]*protocol.Protocol
}

func New() (dispatcher *Dispatcher) {
	return &Dispatcher{
		Events:    NewEvents(),
		protocols: make(map[string]*protocol.Protocol),
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
	protocol = p.protocols[p.ChainID(block.ECR())]

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
