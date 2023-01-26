package chainmanager

import (
	"fmt"
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
)

type Manager struct {
	Events              *Events
	SnapshotCommitment  *Commitment
	CommitmentRequester *eventticker.EventTicker[commitment.ID]

	//TODO: change to memstorage.EpochStorage?
	commitmentsByID      map[commitment.ID]*Commitment
	commitmentsByIDMutex sync.Mutex

	commitmentsByForkingPoint *memstorage.EpochStorage[commitment.ID, *ForkDetectedEvent]

	optsCommitmentRequester []options.Option[eventticker.EventTicker[commitment.ID]]

	commitmentEntityMutex *syncutils.DAGMutex[commitment.ID]
}

func NewManager(snapshot *commitment.Commitment) (manager *Manager) {
	manager = &Manager{
		Events: NewEvents(),

		commitmentsByID:           make(map[commitment.ID]*Commitment),
		commitmentEntityMutex:     syncutils.NewDAGMutex[commitment.ID](),
		commitmentsByForkingPoint: memstorage.NewEpochStorage[commitment.ID, *ForkDetectedEvent](),
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

func (m *Manager) ProcessCommitmentFromSource(commitment *commitment.Commitment, source identity.ID) (isSolid bool, chain *Chain, wasForked bool) {
	isSolid, chain, wasForked, chainCommitment, commitmentPublished := m.registerCommitment(commitment, false)
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

	if wasForked {
		m.handleFork(commitment, chain, source)
	}

	return isSolid, chain, wasForked
}

func (m *Manager) handleFork(commitment *commitment.Commitment, chain *Chain, source identity.ID) {
	commitmentsStorage := m.commitmentsByForkingPoint.Get(commitment.Index(), true)

	// Do not trigger another event for the same forking point.
	if commitmentsStorage.Has(chain.ForkingPoint.ID()) {
		return
	}

	// The forking point should be at least 3 epochs in the past w.r.t this commitment index.
	//TODO: replace 3 with values depending on the configuration
	latestChainCommitment := chain.LatestCommitment()
	if latestChainCommitment.ID().Index()-chain.ForkingPoint.ID().Index() < 3 {
		return
	}

	// commitment.Index is the index you request attestations for.
	event := &ForkDetectedEvent{
		Source:     source,
		Commitment: latestChainCommitment.Commitment(),
		Chain:      chain,
	}
	commitmentsStorage.Set(chain.ForkingPoint.ID(), event)
	m.Events.ForkDetected.Trigger(event)
}

func (m *Manager) ProcessCommitment(commitment *commitment.Commitment) {
	if isSolid, chain, wasForked, _, _ := m.registerCommitment(commitment, true); !isSolid || chain == nil || wasForked {
		panic(fmt.Sprintf("invalid commitment %s %s generated by ourselves: isSolid %t chain %v wasForked %t", commitment.ID(), commitment, isSolid, chain, wasForked))
	}
}

func (m *Manager) registerCommitment(commitment *commitment.Commitment, ownCommitment bool) (isSolid bool, chain *Chain, wasForked bool, chainCommitment *Commitment, commitmentPublished bool) {
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

	isSolid, chain, wasForked = m.registerChild(parentCommitment, chainCommitment, ownCommitment)
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

// ForkedEventByForkingPoint returns the forking event generated by a peer for the given forking point.
func (m *Manager) ForkedEventByForkingPoint(commitmentID commitment.ID) (forkedEvent *ForkDetectedEvent, exists bool) {
	if storage := m.commitmentsByForkingPoint.Get(commitmentID.Index(), false); storage != nil {
		return storage.Get(commitmentID)
	}

	return
}

func (m *Manager) Evict(index epoch.Index) {
	//TODO: do we need to evict more stuff?
	m.commitmentsByForkingPoint.Evict(index)
}

func (m *Manager) registerChild(parent *Commitment, child *Commitment, ownCommitment bool) (isSolid bool, chain *Chain, wasForked bool) {
	if parent.ID() == m.SnapshotCommitment.Chain().LatestCommitment().ID() && !ownCommitment {
		return parent.IsSolid(), parent.Chain(), false
	}

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
