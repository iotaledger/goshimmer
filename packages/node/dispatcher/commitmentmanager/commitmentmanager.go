package commitmentmanager

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type CommitmentManager struct {
	Events             *Events
	SnapshotCommitment *Commitment

	commitmentsByEC map[epoch.EC]*Commitment
	dagMutex        *syncutils.DAGMutex[epoch.EC]

	sync.Mutex
}

func New(snapshotIndex epoch.Index, snapshotECR epoch.ECR, snapshotPrevECR epoch.EC) (manager *CommitmentManager) {
	manager = &CommitmentManager{
		Events: NewEvents(),

		commitmentsByEC: make(map[epoch.EC]*Commitment),
		dagMutex:        syncutils.NewDAGMutex[epoch.EC](),
	}

	manager.SnapshotCommitment = manager.Commitment(NewEC(snapshotIndex, snapshotECR, snapshotPrevECR), true)
	manager.SnapshotCommitment.publishECRecord(snapshotIndex, snapshotECR, snapshotPrevECR)
	manager.SnapshotCommitment.publishChain(NewChain(manager.SnapshotCommitment))

	manager.commitmentsByEC[manager.SnapshotCommitment.EC] = manager.SnapshotCommitment

	return
}

func (c *CommitmentManager) ProcessCommitment(index epoch.Index, ecr epoch.ECR, prevEC epoch.EC) (chain *Chain, wasForked bool) {
	commitment := c.Commitment(NewEC(index, ecr, prevEC), true)
	if !commitment.publishECRecord(index, ecr, prevEC) {
		return commitment.Chain(), false
	}

	if chain, wasForked = c.registerChild(prevEC, commitment); chain == nil {
		return
	}

	if wasForked {
		c.Events.ForkDetected.Trigger(chain)
	}

	if children := commitment.Children(); len(children) != 0 {
		for childWalker := walker.New[*Commitment]().Push(children[0]); childWalker.HasNext(); {
			childWalker.PushAll(c.propagateChainToFirstChild(childWalker.Next(), chain)...)
		}
	}

	return
}

func (c *CommitmentManager) Chain(ec epoch.EC) (chain *Chain) {
	if commitment := c.Commitment(ec, false); commitment != nil {
		return commitment.Chain()
	}

	return
}

func (c *CommitmentManager) Commitment(ec epoch.EC, createIfAbsent ...bool) (commitment *Commitment) {
	c.Lock()
	defer c.Unlock()

	commitment, exists := c.commitmentsByEC[ec]
	if !exists && len(createIfAbsent) >= 1 && createIfAbsent[0] {
		commitment = NewCommitment(ec)
		c.commitmentsByEC[ec] = commitment
	}

	return
}

func (c *CommitmentManager) Commitments(ec epoch.EC, amount int) (commitments []*Commitment, err error) {
	c.Lock()
	defer c.Unlock()

	commitments = make([]*Commitment, amount)

	for i := 0; i < amount; i++ {
		commitment, exists := c.commitmentsByEC[ec]
		if !exists {
			return nil, errors.Errorf("not all commitments in the given range are known")
		}

		commitments[i] = commitment

		ec = commitment.PrevEC()
	}

	return
}

func (c *CommitmentManager) registerChild(parent epoch.EC, child *Commitment) (chain *Chain, wasForked bool) {
	c.dagMutex.Lock(child.EC)
	defer c.dagMutex.Unlock(child.EC)

	if chain, wasForked = c.Commitment(parent, true).registerChild(child); chain != nil {
		chain.addCommitment(child)
		child.publishChain(chain)
	}

	return
}

func (c *CommitmentManager) propagateChainToFirstChild(child *Commitment, chain *Chain) (childrenToUpdate []*Commitment) {
	c.dagMutex.Lock(child.EC)
	c.dagMutex.Unlock(child.EC)

	if !child.publishChain(chain) {
		return
	}

	chain.addCommitment(child)

	children := child.Children()
	if len(children) == 0 {
		return
	}

	return []*Commitment{children[0]}
}
