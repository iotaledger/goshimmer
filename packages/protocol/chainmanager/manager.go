package chainmanager

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Manager struct {
	Events             *Events
	SnapshotCommitment *Commitment

	commitmentsByID map[commitment.ID]*Commitment

	sync.Mutex
}

func NewManager(snapshot *commitment.Commitment) (manager *Manager) {
	manager = &Manager{
		Events: NewEvents(),

		commitmentsByID: make(map[commitment.ID]*Commitment),
	}

	manager.SnapshotCommitment = manager.Commitment(snapshot.ID(), true)
	manager.SnapshotCommitment.PublishCommitment(snapshot)
	manager.SnapshotCommitment.publishChain(NewChain(manager.SnapshotCommitment))

	manager.commitmentsByID[manager.SnapshotCommitment.ID()] = manager.SnapshotCommitment

	return
}

func (c *Manager) ProcessCommitment(commitment *commitment.Commitment) (chain *Chain, wasForked bool) {
	chainCommitment := c.Commitment(commitment.ID(), true)
	if !chainCommitment.PublishCommitment(commitment) {
		return chainCommitment.Chain(), false
	}

	if chain, wasForked = c.registerChild(chainCommitment.Commitment().PrevID(), chainCommitment); chain == nil {
		// TODO: CHAIN SOLIDIFICATION
		return
	}

	if wasForked {
		c.Events.ForkDetected.Trigger(chain)
	}

	if children := chainCommitment.Children(); len(children) != 0 {
		for childWalker := walker.New[*Commitment]().Push(children[0]); childWalker.HasNext(); {
			childWalker.PushAll(c.propagateChainToFirstChild(childWalker.Next(), chain)...)
		}
	}

	return
}

func (c *Manager) Chain(ec commitment.ID) (chain *Chain) {
	if commitment := c.Commitment(ec, false); commitment != nil {
		return commitment.Chain()
	}

	return
}

func (c *Manager) Commitment(id commitment.ID, createIfAbsent ...bool) (commitment *Commitment) {
	c.Lock()
	defer c.Unlock()

	commitment, exists := c.commitmentsByID[id]
	if !exists && len(createIfAbsent) >= 1 && createIfAbsent[0] {
		commitment = NewCommitment(id)
		c.commitmentsByID[id] = commitment
	}

	return
}

func (c *Manager) Commitments(id commitment.ID, amount int) (commitments []*Commitment, err error) {
	c.Lock()
	defer c.Unlock()

	commitments = make([]*Commitment, amount)

	for i := 0; i < amount; i++ {
		currentCommitment, exists := c.commitmentsByID[id]
		if !exists {
			return nil, errors.Errorf("not all commitments in the given range are known")
		}

		commitments[i] = currentCommitment

		id = currentCommitment.Commitment().PrevID()
	}

	return
}

func (c *Manager) registerChild(parent commitment.ID, child *Commitment) (chain *Chain, wasForked bool) {
	child.lockEntity()
	defer child.unlockEntity()

	if chain, wasForked = c.Commitment(parent, true).registerChild(child); chain != nil {
		chain.addCommitment(child)
		child.publishChain(chain)
	}

	return
}

func (c *Manager) propagateChainToFirstChild(child *Commitment, chain *Chain) (childrenToUpdate []*Commitment) {
	child.lockEntity()
	defer child.unlockEntity()

	if !child.publishChain(chain) {
		return
	}

	chain.addCommitment(child)

	children := child.Children()
	if len(children) == 0 {
		return
	}

	return children[:1]
}
