package chainmanager

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

var (
	ErrCommitmentUnknown  = errors.New("unknown commitment")
	ErrCommitmentNotSolid = errors.New("commitment not solid")
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
	manager.Events.CommitmentMissing.Hook(event.NewClosure(manager.CommitmentRequester.StartTicker))
	manager.Events.MissingCommitmentReceived.Hook(event.NewClosure(manager.CommitmentRequester.StopTicker))

	manager.commitmentsByID[manager.SnapshotCommitment.ID()] = manager.SnapshotCommitment

	return
}

func (m *Manager) processCommitment(commitment *commitment.Commitment) (isNew bool, isSolid bool, wasForked bool, chainCommitment *Commitment) {
	isNew, isSolid, wasForked, chainCommitment = m.registerCommitment(commitment)
	if !isNew || chainCommitment.Chain() == nil {
		return
	}

	// Lock access to the chainCommitment so no children are added while we are propagating solidity
	m.commitmentEntityMutex.Lock(chainCommitment.ID())
	defer m.commitmentEntityMutex.Unlock(chainCommitment.ID())

	if mainChild := chainCommitment.mainChild(); mainChild != nil {
		for childWalker := walker.New[*Commitment]().Push(chainCommitment.mainChild()); childWalker.HasNext(); {
			childWalker.PushAll(m.propagateChainToMainChild(childWalker.Next(), chainCommitment.Chain())...)
		}
	}

	if isSolid {
		if children := chainCommitment.Children(); len(children) != 0 {
			for childWalker := walker.New[*Commitment]().PushAll(children...); childWalker.HasNext(); {
				childWalker.PushAll(m.propagateSolidity(childWalker.Next())...)
			}
		}
	}

	return
}

func (m *Manager) ProcessCommitmentFromSource(commitment *commitment.Commitment, source identity.ID) (isSolid bool, wasForked bool, chain *Chain) {
	_, isSolid, wasForked, chainCommitment := m.processCommitment(commitment)

	if wasForked {
		m.handleFork(chainCommitment, chainCommitment.Chain(), source)
	}

	return isSolid, wasForked, chainCommitment.Chain()
}

func (m *Manager) ProcessCandidateCommitment(commitment *commitment.Commitment) (isSolid bool, wasForked bool, chain *Chain) {
	_, isSolid, wasForked, chainCommitment := m.processCommitment(commitment)
	return isSolid, wasForked, chainCommitment.Chain()
}

func (m *Manager) ProcessCommitment(commitment *commitment.Commitment) (isSolid bool, wasForked bool, chain *Chain) {
	isNew, isSolid, wasForked, chainCommitment := m.processCommitment(commitment)

	if !isNew || wasForked {
		if err := m.switchMainChainToCommitment(chainCommitment); err != nil {
			panic(err)
		}
	}

	return isSolid, false, chainCommitment.Chain()
}

func (m *Manager) handleFork(commitment *Commitment, chain *Chain, source identity.ID) {
	commitmentsStorage := m.commitmentsByForkingPoint.Get(commitment.ID().Index(), true)

	forkingPoint, err := m.forkingPointAgainstMainChain(commitment)
	if err != nil {
		return
	}

	if forkingPoint == nil {
		return
	}

	// Do not trigger another event for the same forking point.
	if commitmentsStorage.Has(forkingPoint.ID()) {
		return
	}

	// The forking point should be at least 3 epochs in the past w.r.t this commitment index.
	//TODO: replace 3 with values depending on the configuration
	latestChainCommitment := chain.LatestCommitment()
	if latestChainCommitment.ID().Index()-forkingPoint.ID().Index() < 3 {
		return
	}

	// commitment.Index is the index you request attestations for.
	event := &ForkDetectedEvent{
		Source:                       source,
		Commitment:                   latestChainCommitment.Commitment(),
		ForkingPointAgainstMainChain: forkingPoint.Commitment(),
	}
	commitmentsStorage.Set(forkingPoint.ID(), event)
	m.Events.ForkDetected.Trigger(event)
}

func (m *Manager) registerCommitment(commitment *commitment.Commitment) (isNew bool, isSolid bool, wasForked bool, chainCommitment *Commitment) {
	m.commitmentEntityMutex.Lock(commitment.ID())
	defer m.commitmentEntityMutex.Unlock(commitment.ID())

	// Lock access to the parent commitment
	m.commitmentEntityMutex.Lock(commitment.PrevID())
	defer m.commitmentEntityMutex.Unlock(commitment.PrevID())

	parentCommitment, commitmentCreated := m.Commitment(commitment.PrevID(), true)
	if commitmentCreated {
		m.Events.CommitmentMissing.Trigger(parentCommitment.ID())
	}

	chainCommitment, created := m.Commitment(commitment.ID(), true)

	if !chainCommitment.PublishCommitment(commitment) {
		return false, chainCommitment.IsSolid(), false, chainCommitment
	}

	if !created {
		m.Events.MissingCommitmentReceived.Trigger(chainCommitment.ID())
	}

	isSolid, _, wasForked = m.registerChild(parentCommitment, chainCommitment)
	return true, isSolid, wasForked, chainCommitment
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
			return nil, errors.Wrap(ErrCommitmentUnknown, "not all commitments in the given range are known")
		}

		commitments[i] = currentCommitment

		id = currentCommitment.Commitment().PrevID()
	}

	return
}

func (m *Manager) forkingPointAgainstMainChain(commitment *Commitment) (*Commitment, error) {
	if !commitment.IsSolid() || commitment.Chain() == nil {
		return nil, errors.Wrapf(ErrCommitmentNotSolid, "commitment %s is not solid", commitment)
	}

	var forkingCommitment *Commitment
	// Walk all possible forks until we reach our main chain by jumping over each forking point
	for chain := commitment.Chain(); chain != m.SnapshotCommitment.Chain(); chain = commitment.Chain() {
		forkingCommitment = chain.ForkingPoint

		if commitment, _ = m.Commitment(forkingCommitment.Commitment().PrevID()); commitment == nil {
			return nil, errors.Wrapf(ErrCommitmentUnknown, "unknown parent of solid commitment %s", forkingCommitment.Commitment().ID())
		}
	}

	return forkingCommitment, nil
}

// ForkedEventByForkingPoint returns the forking event generated by a peer for the given forking point.
func (m *Manager) ForkedEventByForkingPoint(commitmentID commitment.ID) (forkedEvent *ForkDetectedEvent, exists bool) {
	if storage := m.commitmentsByForkingPoint.Get(commitmentID.Index(), false); storage != nil {
		return storage.Get(commitmentID)
	}

	return
}

func (m *Manager) SwitchMainChain(head commitment.ID) error {
	commitment, _ := m.Commitment(head)
	if commitment == nil {
		return errors.Wrapf(ErrCommitmentUnknown, "unknown commitment %s", head)
	}

	return m.switchMainChainToCommitment(commitment)
}

func (m *Manager) switchMainChainToCommitment(commitment *Commitment) error {
	forkingPoint, err := m.forkingPointAgainstMainChain(commitment)
	if err != nil {
		return err
	}

	//  commitment is already part of the main chain
	if forkingPoint == nil {
		return nil
	}

	parentCommitment, _ := m.Commitment(forkingPoint.Commitment().PrevID())
	if parentCommitment == nil {
		return errors.Wrapf(ErrCommitmentUnknown, "unknown parent of solid commitment %s", forkingPoint.ID())
	}

	// Separate the main chain by remove it from the parent
	oldMainCommitment := parentCommitment.mainChild()

	// For each forking point coming out of the main chain we need to reorg the children
	for fp := commitment.Chain().ForkingPoint; ; {
		fpParent, _ := m.Commitment(fp.Commitment().PrevID())

		mainChild := fpParent.mainChild()
		newChildChain := NewChain(mainChild)

		if err := fpParent.setMainChild(fp); err != nil {
			return err
		}

		for childWalker := walker.New[*Commitment]().Push(mainChild); childWalker.HasNext(); {
			childWalker.PushAll(m.propagateReplaceChainToMainChild(childWalker.Next(), newChildChain)...)
		}

		if fp == forkingPoint {
			break
		}

		fp = fpParent.Chain().ForkingPoint
	}

	// Delete all commitments after newer than our new head if the chain was longer than the new one
	snapshotChain := m.SnapshotCommitment.Chain()
	snapshotChain.dropCommitmentsAfter(commitment.ID().Index())

	// Separate the old main chain by removing it from the parent
	parentCommitment.deleteChild(oldMainCommitment)

	// Cleanup the old tree hanging from the old main chain from our cache
	if children := oldMainCommitment.Children(); len(children) != 0 {
		for childWalker := walker.New[*Commitment]().PushAll(children...); childWalker.HasNext(); {
			childWalker.PushAll(m.deleteAllChildrenFromCache(childWalker.Next())...)
		}
	}

	return nil
}

func (m *Manager) Evict(index epoch.Index) {
	//TODO: do we need to evict more stuff?
	m.commitmentsByForkingPoint.Evict(index)
}

func (m *Manager) registerChild(parent *Commitment, child *Commitment) (isSolid bool, chain *Chain, wasForked bool) {
	if isSolid, chain, wasForked = parent.registerChild(child); chain != nil {
		chain.addCommitment(child)
		child.publishChain(chain)
		child.SetSolid(isSolid)
	}

	return
}

func (m *Manager) propagateChainToMainChild(child *Commitment, chain *Chain) (childrenToUpdate []*Commitment) {
	m.commitmentEntityMutex.Lock(child.ID())
	defer m.commitmentEntityMutex.Unlock(child.ID())

	if !child.publishChain(chain) {
		return
	}

	chain.addCommitment(child)

	mainChild := child.mainChild()
	if mainChild == nil {
		return
	}

	return []*Commitment{mainChild}
}

func (m *Manager) propagateReplaceChainToMainChild(child *Commitment, chain *Chain) (childrenToUpdate []*Commitment) {
	m.commitmentEntityMutex.Lock(child.ID())
	defer m.commitmentEntityMutex.Unlock(child.ID())

	child.replaceChain(chain)
	chain.addCommitment(child)

	mainChild := child.mainChild()
	if mainChild == nil {
		return
	}

	return []*Commitment{mainChild}
}

func (m *Manager) deleteAllChildrenFromCache(child *Commitment) (childrenToUpdate []*Commitment) {
	m.commitmentEntityMutex.Lock(child.ID())
	defer m.commitmentEntityMutex.Unlock(child.ID())

	delete(m.commitmentsByID, child.ID())

	return child.Children()
}

func (m *Manager) propagateSolidity(child *Commitment) (childrenToUpdate []*Commitment) {
	m.commitmentEntityMutex.Lock(child.ID())
	defer m.commitmentEntityMutex.Unlock(child.ID())

	if child.SetSolid(true) {
		childrenToUpdate = child.Children()
	}

	return
}
