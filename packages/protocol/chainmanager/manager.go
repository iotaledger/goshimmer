package chainmanager

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/core/eventticker"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

var (
	ErrCommitmentUnknown  = errors.New("unknown commitment")
	ErrCommitmentNotSolid = errors.New("commitment not solid")
)

type Manager struct {
	Events              *Events
	CommitmentRequester *eventticker.EventTicker[commitment.ID]

	commitmentsByID *memstorage.SlotStorage[commitment.ID, *Commitment]
	rootCommitment  *Commitment

	// This tracks the forkingPoints by the commitment that triggered the detection so we can clean up after eviction
	forkingPointsByCommitments *memstorage.SlotStorage[commitment.ID, commitment.ID]
	forksByForkingPoint        *shrinkingmap.ShrinkingMap[commitment.ID, *Fork]

	evictionMutex sync.RWMutex

	optsCommitmentRequester []options.Option[eventticker.EventTicker[commitment.ID]]

	optsMinimumForkDepth int64

	commitmentEntityMutex *syncutils.DAGMutex[commitment.ID]
	lastEvictedSlot       slot.Index
}

func NewManager(opts ...options.Option[Manager]) (manager *Manager) {
	return options.Apply(&Manager{
		Events:               NewEvents(),
		optsMinimumForkDepth: 3,

		commitmentsByID:            memstorage.NewSlotStorage[commitment.ID, *Commitment](),
		commitmentEntityMutex:      syncutils.NewDAGMutex[commitment.ID](),
		forkingPointsByCommitments: memstorage.NewSlotStorage[commitment.ID, commitment.ID](),
		forksByForkingPoint:        shrinkingmap.New[commitment.ID, *Fork](),
		lastEvictedSlot:            slot.Index(-1),
	}, opts, func(m *Manager) {
		m.CommitmentRequester = eventticker.New(m.optsCommitmentRequester...)
		m.Events.CommitmentMissing.Hook(m.CommitmentRequester.StartTicker)
		m.Events.MissingCommitmentReceived.Hook(m.CommitmentRequester.StopTicker)
		m.Events.CommitmentBelowRoot.Hook(m.CommitmentRequester.StopTicker)
	})
}

func (m *Manager) Initialize(c *commitment.Commitment) {
	m.evictionMutex.Lock()
	defer m.evictionMutex.Unlock()

	m.rootCommitment, _ = m.getOrCreateCommitment(c.ID())
	m.rootCommitment.PublishCommitment(c)
	m.rootCommitment.SetSolid(true)
	m.rootCommitment.publishChain(NewChain(m.rootCommitment))
}

func (m *Manager) ProcessCommitmentFromSource(commitment *commitment.Commitment, source identity.ID) (isSolid bool, chain *Chain) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	_, isSolid, chainCommitment := m.processCommitment(commitment)
	if chainCommitment == nil {
		return false, nil
	}

	m.detectForks(chainCommitment, source)

	return isSolid, chainCommitment.Chain()
}

func (m *Manager) ProcessCandidateCommitment(commitment *commitment.Commitment) (isSolid bool, chain *Chain) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	_, isSolid, chainCommitment := m.processCommitment(commitment)
	if chainCommitment == nil {
		return false, nil
	}
	return isSolid, chainCommitment.Chain()
}

func (m *Manager) ProcessCommitment(commitment *commitment.Commitment) (isSolid bool, chain *Chain) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	isNew, isSolid, chainCommitment := m.processCommitment(commitment)

	if chainCommitment == nil {
		return false, nil
	}

	if !isNew {
		if err := m.switchMainChainToCommitment(chainCommitment); err != nil {
			panic(err)
		}
	}

	return isSolid, chainCommitment.Chain()
}

func (m *Manager) EvictUntil(index slot.Index) {
	m.evictionMutex.Lock()
	defer m.evictionMutex.Unlock()

	if index <= m.lastEvictedSlot {
		return
	}

	for currentIndex := m.lastEvictedSlot + 1; currentIndex <= index; currentIndex++ {
		m.evict(currentIndex)
	}
	m.lastEvictedSlot = index
}

// RootCommitment returns the root commitment of the manager.
func (m *Manager) RootCommitment() (rootCommitment *Commitment) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	return m.rootCommitment
}

// SetRootCommitment sets the root commitment of the manager.
func (m *Manager) SetRootCommitment(commitment *commitment.Commitment) {
	m.evictionMutex.Lock()
	defer m.evictionMutex.Unlock()

	storage := m.commitmentsByID.Get(commitment.Index())
	if storage == nil {
		panic(fmt.Sprint("we should always have commitment storage for confirmed index", commitment))
	}

	newRootCommitment, exists := storage.Get(commitment.ID())
	if !exists {
		panic(fmt.Sprint("we should always have the latest commitment ID we confirmed with", commitment))
	}

	if commitment.Index() <= m.rootCommitment.Commitment().Index() && commitment.ID() != m.rootCommitment.Commitment().ID() {
		panic(fmt.Sprint("we should never set the root commitment to a commitment that is below the current root commitment", commitment, m.rootCommitment.Commitment()))
	}

	m.rootCommitment = newRootCommitment
}

func (m *Manager) Chain(ec commitment.ID) (chain *Chain) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	if commitment, exists := m.commitment(ec); exists {
		return commitment.Chain()
	}

	return nil
}

func (m *Manager) Commitments(id commitment.ID, amount int) (commitments []*Commitment, err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	commitments = make([]*Commitment, amount)

	for i := 0; i < amount; i++ {
		currentCommitment, _ := m.commitment(id)
		if currentCommitment == nil {
			return nil, errors.Wrap(ErrCommitmentUnknown, "not all commitments in the given range are known")
		}

		commitments[i] = currentCommitment

		id = currentCommitment.Commitment().PrevID()
	}

	return
}

// ForkByForkingPoint returns the fork generated by a peer for the given forking point.
func (m *Manager) ForkByForkingPoint(forkingPoint commitment.ID) (fork *Fork, exists bool) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	return m.forksByForkingPoint.Get(forkingPoint)
}

func (m *Manager) SwitchMainChain(head commitment.ID) error {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	commitment, _ := m.commitment(head)
	if commitment == nil {
		return errors.Wrapf(ErrCommitmentUnknown, "unknown commitment %s", head)
	}

	return m.switchMainChainToCommitment(commitment)
}

func (m *Manager) processCommitment(commitment *commitment.Commitment) (isNew bool, isSolid bool, chainCommitment *Commitment) {
	if isBelowRootCommitment, isRootCommitment := m.evaluateAgainstRootCommitment(commitment); isBelowRootCommitment || isRootCommitment {
		if isRootCommitment {
			chainCommitment = m.rootCommitment
		} else {
			m.Events.CommitmentBelowRoot.Trigger(commitment.ID())
		}
		return false, isRootCommitment, chainCommitment
	}

	isNew, isSolid, _, chainCommitment = m.registerCommitment(commitment)
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

func (m *Manager) evict(index slot.Index) {
	// Forget about the forks that we detected at that slot so that we can detect them again if they happen
	evictedForkDetections := m.forkingPointsByCommitments.Evict(index)
	if evictedForkDetections != nil {
		evictedForkDetections.ForEach(func(_, forkingPoint commitment.ID) bool {
			m.forksByForkingPoint.Delete(forkingPoint)
			return true
		})
	}

	m.commitmentsByID.Evict(index)
}

func (m *Manager) getOrCreateCommitment(id commitment.ID) (commitment *Commitment, created bool) {
	return m.commitmentsByID.Get(id.Index(), true).GetOrCreate(id, func() *Commitment {
		return NewCommitment(id)
	})
}

func (m *Manager) commitment(id commitment.ID) (commitment *Commitment, exists bool) {
	storage := m.commitmentsByID.Get(id.Index())
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (m *Manager) evaluateAgainstRootCommitment(commitment *commitment.Commitment) (isBelow, isRootCommitment bool) {
	isBelow = commitment.Index() <= m.rootCommitment.Commitment().Index()
	isRootCommitment = commitment.Equals(m.rootCommitment.Commitment())

	return
}

func (m *Manager) detectForks(commitment *Commitment, source identity.ID) {
	forkingPoint, err := m.forkingPointAgainstMainChain(commitment)
	if err != nil {
		return
	}

	if forkingPoint == nil {
		return
	}

	// Do not trigger another event for the same forking point.
	if m.forksByForkingPoint.Has(forkingPoint.ID()) {
		return
	}

	// The forking point should be at least optsMinimumForkDepth slots in the past w.r.t this commitment index.
	latestChainCommitment := commitment.Chain().LatestCommitment()
	if int64(latestChainCommitment.ID().Index()-forkingPoint.ID().Index()) < m.optsMinimumForkDepth {
		return
	}

	// commitment.Index is the index you request attestations for.
	fork := &Fork{
		Source:       source,
		Commitment:   latestChainCommitment.Commitment(),
		ForkingPoint: forkingPoint.Commitment(),
	}
	m.forksByForkingPoint.Set(forkingPoint.ID(), fork)

	m.forkingPointsByCommitments.Get(commitment.ID().Index(), true).Set(commitment.ID(), forkingPoint.ID())

	m.Events.ForkDetected.Trigger(fork)
}

func (m *Manager) forkingPointAgainstMainChain(commitment *Commitment) (*Commitment, error) {
	if !commitment.IsSolid() || commitment.Chain() == nil {
		return nil, errors.Wrapf(ErrCommitmentNotSolid, "commitment %s is not solid", commitment)
	}

	var forkingCommitment *Commitment
	// Walk all possible forks until we reach our main chain by jumping over each forking point
	for chain := commitment.Chain(); chain != m.RootCommitment().Chain(); chain = commitment.Chain() {
		forkingCommitment = chain.ForkingPoint

		if commitment, _ = m.commitment(forkingCommitment.Commitment().PrevID()); commitment == nil {
			return nil, errors.Wrapf(ErrCommitmentUnknown, "unknown parent of solid commitment %s", forkingCommitment.Commitment().ID())
		}
	}

	return forkingCommitment, nil
}

func (m *Manager) registerCommitment(commitment *commitment.Commitment) (isNew bool, isSolid bool, wasForked bool, chainCommitment *Commitment) {
	m.commitmentEntityMutex.Lock(commitment.ID())
	defer m.commitmentEntityMutex.Unlock(commitment.ID())

	// Lock access to the parent commitment
	m.commitmentEntityMutex.Lock(commitment.PrevID())
	defer m.commitmentEntityMutex.Unlock(commitment.PrevID())

	parentCommitment, commitmentCreated := m.getOrCreateCommitment(commitment.PrevID())
	if commitmentCreated {
		m.Events.CommitmentMissing.Trigger(parentCommitment.ID())
	}

	chainCommitment, created := m.getOrCreateCommitment(commitment.ID())

	if !chainCommitment.PublishCommitment(commitment) {
		return false, chainCommitment.IsSolid(), false, chainCommitment
	}

	if !created {
		m.Events.MissingCommitmentReceived.Trigger(chainCommitment.ID())
	}

	isSolid, _, wasForked = m.registerChild(parentCommitment, chainCommitment)
	return true, isSolid, wasForked, chainCommitment
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

	parentCommitment, _ := m.commitment(forkingPoint.Commitment().PrevID())
	if parentCommitment == nil {
		return errors.Wrapf(ErrCommitmentUnknown, "unknown parent of solid commitment %s", forkingPoint.ID())
	}

	// Separate the main chain by remove it from the parent
	oldMainCommitment := parentCommitment.mainChild()

	// For each forking point coming out of the main chain we need to reorg the children
	for fp := commitment.Chain().ForkingPoint; ; {
		fpParent, _ := m.commitment(fp.Commitment().PrevID())

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
	snapshotChain := m.rootCommitment.Chain()
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

	if storage := m.commitmentsByID.Get(child.ID().Index()); storage != nil {
		storage.Delete(child.ID())
	}

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

func WithForkDetectionMinimumDepth(depth int64) options.Option[Manager] {
	return func(m *Manager) {
		m.optsMinimumForkDepth = depth
	}
}
