package chainmanager

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
)

type Manager struct {
	Events              *Events
	SnapshotCommitment  *Commitment
	EvictionManager     *eviction.State[commitment.ID]
	CommitmentRequester *eventticker.EventTicker[commitment.ID]

	commitmentsByID map[commitment.ID]*Commitment

	optsCommitmentRequester []options.Option[eventticker.EventTicker[commitment.ID]]

	sync.Mutex
}

func NewManager(snapshot *commitment.Commitment) (manager *Manager) {
	manager = &Manager{
		Events:          NewEvents(),
		EvictionManager: eviction.NewState[commitment.ID](),

		commitmentsByID: make(map[commitment.ID]*Commitment),
	}

	manager.SnapshotCommitment, _ = manager.Commitment(snapshot.ID(), true)
	manager.SnapshotCommitment.PublishCommitment(snapshot)
	manager.SnapshotCommitment.SetSolid(true)
	manager.SnapshotCommitment.publishChain(NewChain(manager.SnapshotCommitment))

	manager.CommitmentRequester = eventticker.New(manager.EvictionManager, manager.optsCommitmentRequester...)
	manager.Events.CommitmentMissing.Attach(event.NewClosure(manager.CommitmentRequester.StartTicker))
	manager.Events.MissingCommitmentReceived.Attach(event.NewClosure(manager.CommitmentRequester.StopTicker))

	manager.commitmentsByID[manager.SnapshotCommitment.ID()] = manager.SnapshotCommitment

	return
}

func (c *Manager) ProcessCommitment(commitment *commitment.Commitment) (isSolid bool, chain *Chain, wasForked bool) {
	chainCommitment, created := c.Commitment(commitment.ID(), true)
	if !chainCommitment.PublishCommitment(commitment) {
		return chainCommitment.IsSolid(), chainCommitment.Chain(), false
	}
	
	if !created {
		c.Events.MissingCommitmentReceived.Trigger(chainCommitment.ID())
	}

	parentCommitment, commitmentCreated := c.Commitment(commitment.PrevID(), true)
	if commitmentCreated {
		c.Events.CommitmentMissing.Trigger(parentCommitment.ID())
	}

	if isSolid, chain, wasForked = c.registerChild(parentCommitment, chainCommitment); chain == nil {
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

	if isSolid {
		if children := chainCommitment.Children(); len(children) != 0 {
			for childWalker := walker.New[*Commitment]().PushAll(children...); childWalker.HasNext(); {
				childWalker.PushAll(c.propagateSolidity(childWalker.Next())...)
			}
		}
	}

	return
}

func (c *Manager) Chain(ec commitment.ID) (chain *Chain) {
	if commitment, _ := c.Commitment(ec, false); commitment != nil {
		return commitment.Chain()
	}

	return
}

func (c *Manager) Commitment(id commitment.ID, createIfAbsent ...bool) (commitment *Commitment, created bool) {
	c.Lock()
	defer c.Unlock()

	commitment, exists := c.commitmentsByID[id]
	if created = !exists && len(createIfAbsent) >= 1 && createIfAbsent[0]; created {
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

func (c *Manager) registerChild(parent *Commitment, child *Commitment) (isSolid bool, chain *Chain, wasForked bool) {
	child.lockEntity()
	defer child.unlockEntity()

	if isSolid, chain, wasForked = parent.registerChild(child); chain != nil {
		chain.addCommitment(child)
		child.publishChain(chain)
		child.solid = isSolid
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

func (c *Manager) propagateSolidity(child *Commitment) (childrenToUpdate []*Commitment) {
	child.lockEntity()
	defer child.unlockEntity()

	if child.SetSolid(true) {
		child.Chain().SetLastSolidIndex(child.Commitment().Index())

		childrenToUpdate = child.Children()
	}

	return
}
