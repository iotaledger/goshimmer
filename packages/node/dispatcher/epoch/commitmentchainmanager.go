package epoch

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type CommitmentChainManager struct {
	GenesisCommitment *Commitment

	commitmentsByID map[CommitmentID]*Commitment
	dagMutex        *syncutils.DAGMutex[CommitmentID]

	sync.Mutex
}

func NewCommitmentChainManager(genesisIndex epoch.Index, genesisECR epoch.ECR) (manager *CommitmentChainManager) {
	manager = &CommitmentChainManager{
		commitmentsByID: make(map[CommitmentID]*Commitment),
		dagMutex:        syncutils.NewDAGMutex[CommitmentID](),
	}

	manager.GenesisCommitment = manager.Commitment(NewCommitmentID(genesisIndex, genesisECR, epoch.EC{}))
	manager.GenesisCommitment.publishECRecord(genesisIndex, genesisECR, epoch.EC{})
	manager.GenesisCommitment.publishChain(NewCommitmentChain(manager.GenesisCommitment))

	manager.commitmentsByID[manager.GenesisCommitment.ID] = manager.GenesisCommitment

	return
}

func (c *CommitmentChainManager) Chain(index epoch.Index, ecr epoch.ECR, prevEC epoch.EC) (chain *CommitmentChain) {
	commitment := c.Commitment(NewCommitmentID(index, ecr, prevEC))
	if !commitment.publishECRecord(index, ecr, prevEC) {
		return commitment.Chain()
	}

	if chain = c.registerChild(prevEC, commitment); chain == nil {
		return
	}

	children := commitment.Children()
	if len(children) == 0 {
		return
	}

	for childWalker := walker.New[*Commitment]().Push(children[0]); childWalker.HasNext(); {
		childWalker.PushAll(c.propagateChainToFirstChild(childWalker.Next(), chain)...)
	}

	return
}

func (c *CommitmentChainManager) Commitment(commitmentID CommitmentID) (commitment *Commitment) {
	c.Lock()
	defer c.Unlock()

	commitment, exists := c.commitmentsByID[commitmentID]
	if !exists {
		commitment = NewCommitment(commitmentID)
		c.commitmentsByID[commitmentID] = commitment
	}

	return
}

func (c *CommitmentChainManager) registerChild(parent CommitmentID, child *Commitment) (chain *CommitmentChain) {
	c.dagMutex.Lock(child.ID)
	defer c.dagMutex.Unlock(child.ID)

	if chain = c.Commitment(parent).registerChild(child); chain != nil {
		chain.addCommitment(child)
		child.publishChain(chain)
	}

	return
}

func (c *CommitmentChainManager) propagateChainToFirstChild(child *Commitment, chain *CommitmentChain) (childrenToUpdate []*Commitment) {
	c.dagMutex.Lock(child.ID)
	c.dagMutex.Unlock(child.ID)

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
