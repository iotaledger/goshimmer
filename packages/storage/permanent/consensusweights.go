package permanent

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type ConsensusWeights struct {
	consensusWeights *ads.Map[identity.ID, models.TimedBalance, *models.TimedBalance]
	store            kvstore.KVStore
}

func NewConsensusWeights(store kvstore.KVStore) (newConsensusWeights *ConsensusWeights) {
	return &ConsensusWeights{
		consensusWeights: ads.NewMap[identity.ID, models.TimedBalance](store),
		store:            store,
	}
}

func (c *ConsensusWeights) Load(id identity.ID) (weight *models.TimedBalance, exists bool) {
	return c.consensusWeights.Get(id)
}

func (c *ConsensusWeights) Store(id identity.ID, weight *models.TimedBalance) {
	c.consensusWeights.Set(id, weight)
}

func (c *ConsensusWeights) Delete(id identity.ID) (deleted bool) {
	return c.consensusWeights.Delete(id)
}

func (c *ConsensusWeights) Root() types.Identifier {
	return c.consensusWeights.Root()
}

// Stream streams the weights of all actors and when they were last updated.
func (c *ConsensusWeights) Stream(callback func(id identity.ID, weight int64, lastUpdated epoch.Index) bool) (err error) {
	if storageErr := c.store.Iterate([]byte{}, func(idBytes kvstore.Key, timedBalanceBytes kvstore.Value) bool {
		var id identity.ID
		if _, err = id.FromBytes(idBytes); err != nil {
			return false
		}

		var timedBalance = new(models.TimedBalance)
		if _, err = timedBalance.FromBytes(timedBalanceBytes); err != nil {
			return false
		}

		return callback(id, timedBalance.Balance, timedBalance.LastUpdated)
	}); storageErr != nil {
		return errors.Errorf("failed to iterate over Consensus Weights: %w", storageErr)
	}

	return err
}
