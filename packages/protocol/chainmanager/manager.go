package chainmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Manager struct {
	Events              *Events
	SnapshotCommitment  *Commitment
	CommitmentRequester *eventticker.EventTicker[commitment.ID]

	commitmentsByID      map[commitment.ID]*Commitment
	commitmentsByIDMutex sync.Mutex

	forkingBlocks *memstorage.EpochStorage[identity.ID, *models.Block]

	optsCommitmentRequester []options.Option[eventticker.EventTicker[commitment.ID]]

	commitmentEntityMutex *syncutils.DAGMutex[commitment.ID]
}

func NewManager(snapshot *commitment.Commitment) (manager *Manager) {
	manager = &Manager{
		Events: NewEvents(),

		commitmentsByID:       make(map[commitment.ID]*Commitment),
		commitmentEntityMutex: syncutils.NewDAGMutex[commitment.ID](),
		forkingBlocks:         memstorage.NewEpochStorage[identity.ID, *models.Block](),
	}

	manager.SnapshotCommitment, _ = manager.Commitment(snapshot.ID(), true)
	manager.SnapshotCommitment.PublishCommitment(snapshot)
	manager.SnapshotCommitment.SetSolid(true)
	manager.SnapshotCommitment.publishChain(NewChain(manager.SnapshotCommitment))

	manager.CommitmentRequester = eventticker.New(manager.optsCommitmentRequester...)
	manager.Events.CommitmentMissing.Attach(event.NewClosure(manager.CommitmentRequester.StartTicker))
	manager.Events.MissingCommitmentReceived.Attach(event.NewClosure(manager.CommitmentRequester.StopTicker))

	manager.commitmentsByID[manager.SnapshotCommitment.ID()] = manager.SnapshotCommitment

	return
}

func (m *Manager) ProcessBlock(block *models.Block, source identity.ID) (isSolid bool, chain *Chain, wasForked bool) {
	if isSolid, chain, wasForked = m.ProcessCommitment(block.Commitment()); wasForked {
		m.forkingBlocks.Get(block.Commitment().Index(), true).Set(source, block)
		m.Events.ForkDetected.Trigger(&ForkDetectedEvent{
			Chain:     chain,
			ClaimedCW: block.Commitment().CumulativeWeight(),
		})
	}

	return
}

func (m *Manager) ProcessCommitment(commitment *commitment.Commitment) (isSolid bool, chain *Chain, wasForked bool) {
	// TODO: do not extend the main chain if the commitment doesn't come from myself, after I am bootstrapped.
	isSolid, chain, wasForked, chainCommitment, commitmentPublished := m.registerCommitment(commitment)
	if !commitmentPublished || chain == nil {
		return isSolid, chain, wasForked
	}

	// Lock access to the chainCommitment so no children are added while we are propagating solidity
	m.commitmentEntityMutex.Lock(chainCommitment.ID())
	defer m.commitmentEntityMutex.Unlock(chainCommitment.ID())

	if children := chainCommitment.Children(); len(children) != 0 {
		for childWalker := walker.New[*Commitment]().Push(children[0]); childWalker.HasNext(); {
			childWalker.PushAll(m.propagateChainToFirstChild(childWalker.Next(), chain)...)
		}
	}

	if isSolid {
		if children := chainCommitment.Children(); len(children) != 0 {
			for childWalker := walker.New[*Commitment]().PushAll(children...); childWalker.HasNext(); {
				childWalker.PushAll(m.propagateSolidity(childWalker.Next())...)
			}
		}
	}

	return isSolid, chain, wasForked
}

func (m *Manager) registerCommitment(commitment *commitment.Commitment) (isSolid bool, chain *Chain, wasForked bool, chainCommitment *Commitment, commitmentPublished bool) {
	m.commitmentEntityMutex.Lock(commitment.ID())
	defer m.commitmentEntityMutex.Unlock(commitment.ID())

	chainCommitment, created := m.Commitment(commitment.ID(), true)

	if !chainCommitment.PublishCommitment(commitment) {
		return chainCommitment.IsSolid(), chainCommitment.Chain(), false, chainCommitment, false
	}

	if !created {
		m.Events.MissingCommitmentReceived.Trigger(chainCommitment.ID())
	}

	// Lock access to the parent commitment
	m.commitmentEntityMutex.Lock(commitment.PrevID())
	defer m.commitmentEntityMutex.Unlock(commitment.PrevID())

	parentCommitment, commitmentCreated := m.Commitment(commitment.PrevID(), true)
	if commitmentCreated {
		m.Events.CommitmentMissing.Trigger(parentCommitment.ID())
	}

	isSolid, chain, wasForked = m.registerChild(parentCommitment, chainCommitment)
	return isSolid, chain, wasForked, chainCommitment, true
}

func (m *Manager) Chain(ec commitment.ID) (chain *Chain) {
	if commitment, _ := m.Commitment(ec, false); commitment != nil {
		return commitment.Chain()
	}

	return
}

func (m *Manager) Commitment(id commitment.ID, createIfAbsent ...bool) (commitment *Commitment, created bool) {
	m.commitmentsByIDMutex.Lock()
	defer m.commitmentsByIDMutex.Unlock()

	commitment, exists := m.commitmentsByID[id]
	if created = !exists && len(createIfAbsent) >= 1 && createIfAbsent[0]; created {
		commitment = NewCommitment(id)
		m.commitmentsByID[id] = commitment
	}

	return
}

func (m *Manager) Commitments(id commitment.ID, amount int) (commitments []*Commitment, err error) {
	m.commitmentsByIDMutex.Lock()
	defer m.commitmentsByIDMutex.Unlock()

	commitments = make([]*Commitment, amount)

	for i := 0; i < amount; i++ {
		currentCommitment, exists := m.commitmentsByID[id]
		if !exists {
			return nil, errors.New("not all commitments in the given range are known")
		}

		commitments[i] = currentCommitment

		id = currentCommitment.Commitment().PrevID()
	}

	return
}

// ForkingBlockFromSource returns the block containing a forking commitment on an index, received by the given source.
func (m *Manager) ForkingBlockFromSource(index epoch.Index, source identity.ID) (block *models.Block, exists bool) {
	if storage := m.forkingBlocks.Get(index, false); storage != nil {
		return storage.Get(source)
	}

	return
}

func (m *Manager) registerChild(parent *Commitment, child *Commitment) (isSolid bool, chain *Chain, wasForked bool) {
	if isSolid, chain, wasForked = parent.registerChild(child); chain != nil {
		chain.addCommitment(child)
		child.publishChain(chain)
		child.SetSolid(isSolid)
	}

	return
}

func (m *Manager) propagateChainToFirstChild(child *Commitment, chain *Chain) (childrenToUpdate []*Commitment) {
	m.commitmentEntityMutex.Lock(child.ID())
	defer m.commitmentEntityMutex.Unlock(child.ID())

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

func (m *Manager) propagateSolidity(child *Commitment) (childrenToUpdate []*Commitment) {
	m.commitmentEntityMutex.Lock(child.ID())
	defer m.commitmentEntityMutex.Unlock(child.ID())

	if child.SetSolid(true) {
		childrenToUpdate = child.Children()
	}

	return
}
