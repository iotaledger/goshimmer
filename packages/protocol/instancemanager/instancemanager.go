package instancemanager

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/models"
)

type InstanceManager struct {
	Events *Events

	activeInstance     *instance.Instance
	instancesByChainID map[commitment.ID]*instance.Instance
	chainManager       *chainmanager.Manager
}

func New(snapshotCommitment *commitment.Commitment) (manager *InstanceManager) {
	manager = &InstanceManager{
		Events: NewEvents(),

		instancesByChainID: make(map[commitment.ID]*instance.Instance),
		chainManager:       chainmanager.NewManager(snapshotCommitment),
	}

	// todo try to instantiate main protocol

	manager.Events.Instance.LinkTo(manager.activeInstance.Events)

	return manager
}

func (p *InstanceManager) DispatchBlockData(bytes []byte, neighbor *p2p.Neighbor) {
	block := new(models.Block)
	if _, err := block.FromBytes(bytes); err != nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	chain, wasForked := p.chainManager.ProcessCommitment(block.EI(), block.ECR(), block.PrevEC())
	if chain == nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	fmt.Println(wasForked)

	return
}