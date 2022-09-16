package chainmanager

import (
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/models"
)

type Chain struct {
	ForkingPoint *Commitment
	// Metadatastorage
	BlockStorage *database.PersistentEpochStorage[models.BlockID, models.Block, *models.BlockID, *models.Block]

	latestCommittableEpoch epoch.Index
	commitmentsByIndex     map[epoch.Index]*Commitment

	sync.RWMutex
}

func NewChain(forkingPoint *Commitment) (fork *Chain) {
	return &Chain{
		ForkingPoint: forkingPoint,

		commitmentsByIndex: map[epoch.Index]*Commitment{
			forkingPoint.Index(): forkingPoint,
		},
	}
}

func (c *Chain) BlocksCount(index epoch.Index) (blocksCount int) {
	return 0
}

func (c *Chain) StreamEpochBlocks(index epoch.Index, callback func(blocks []*models.Block), batchSize int) (err error) {
	c.RLock()
	defer c.RUnlock()

	if index > c.latestCommittableEpoch {
		return errors.Errorf("cannot stream blocks of epoch %d: not committable yet", index)
	}

	blocks := make([]*models.Block, 0)
	if err = c.BlockStorage.Iterate(index, func(key models.BlockID, value *models.Block) bool {
		value.SetID(key)

		blocks = append(blocks, value)

		if len(blocks) == batchSize {
			callback(blocks)
			blocks = make([]*models.Block, 0)
		}

		return true
	}); err != nil {
		return errors.Errorf("failed to stream epoch blocks: %w", err)
	}

	if len(blocks) > 0 {
		callback(blocks)
	}

	return
}

func (c *Chain) Commitment(index epoch.Index) (commitment *Commitment) {
	c.RLock()
	defer c.RUnlock()

	return c.commitmentsByIndex[index]
}

func (c *Chain) Size() int {
	c.RLock()
	defer c.RUnlock()

	return len(c.commitmentsByIndex)
}

func (c *Chain) addCommitment(commitment *Commitment) {
	c.Lock()
	defer c.Unlock()

	c.commitmentsByIndex[commitment.Index()] = commitment
}
