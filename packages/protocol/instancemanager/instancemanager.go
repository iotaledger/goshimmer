package instancemanager

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type InstanceManager struct {
	Events *Events

	activeInstance     *instance.Instance
	instancesByChainID map[commitment.ID]*instance.Instance
	chainManager       *chainmanager.Manager
}

func New(diskUtil *diskutil.DiskUtil, log *logger.Logger) (manager *InstanceManager) {
	var snapshotCommitment *commitment.Commitment

	manager = &InstanceManager{
		Events: NewEvents(),

		instancesByChainID: make(map[commitment.ID]*instance.Instance),
		chainManager:       chainmanager.NewManager(snapshotCommitment),
	}

	// create WorkingDirectory for new chain (if not exists)
	// copy over current snapshot to that working directory (if it doesn't exist yet)
	// start new instance with that working directory
	// add instance to instancesByChainID

	// todo try to instantiate main protocol
	manager.activeInstance = instance.New(nil, log)

	manager.Events.Instance.LinkTo(manager.activeInstance.Events)

	return manager
}

func (p *InstanceManager) DispatchBlockData(bytes []byte, neighbor *p2p.Neighbor) {
	block := new(models.Block)
	if _, err := block.FromBytes(bytes); err != nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	chain, wasForked := p.chainManager.ProcessCommitment(block.Commitment())
	if chain == nil {
		p.Events.InvalidBlockReceived.Trigger(neighbor)
		return
	}

	fmt.Println(wasForked)

	return
}
