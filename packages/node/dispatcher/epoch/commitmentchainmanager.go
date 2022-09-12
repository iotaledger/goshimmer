package epoch

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type CommitmentChainManager struct {
	commitmentsByID map[CommitmentID]*Commitment
	dagMutex        *syncutils.DAGMutex[CommitmentID]

	sync.Mutex
}

func NewCommitmentChainManager(genesisCommitmentID CommitmentID) (commitmentChainManager *CommitmentChainManager) {
	genesisCommitment := NewCommitment(genesisCommitmentID)
	genesisCommitment.publishChain(NewCommitmentChain(genesisCommitment))

	return &CommitmentChainManager{
		commitmentsByID: map[CommitmentID]*Commitment{
			genesisCommitment.ID: genesisCommitment,
		},
		dagMutex: syncutils.NewDAGMutex[CommitmentID](),
	}
}

func (c *CommitmentChainManager) ChainFromBlock(block *models.Block) (chain *CommitmentChain) {
	commitment, wasPublished := c.publishBlock(block)
	if !wasPublished {
		return commitment.Chain()
	}

	if chain = c.registerChild(block.PrevEC(), commitment); chain == nil {
		return
	}

	if children := commitment.Children(); len(children) != 0 {
		for childWalker := walker.New[*Commitment]().PushAll(commitment.Children()...); childWalker.HasNext(); {
			childWalker.PushAll(c.propagateChainToFirstChild(childWalker.Next(), chain)...)
		}
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

func (c *CommitmentChainManager) publishBlock(block *models.Block) (commitment *Commitment, wasPublished bool) {
	commitment = c.Commitment(epoch.NewEpochCommitment(block.EI(), block.ECR(), block.PrevEC()))

	return commitment, commitment.publishECRecord(block.EI(), block.ECR(), block.PrevEC())
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
