package permanent

import (
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type ConsensusWeights struct {
	consensusWeights *ads.Map[identity.ID, models.TimedBalance, *models.TimedBalance]
}

func NewConsensusWeights(store kvstore.KVStore) (newConsensusWeights *ConsensusWeights) {
	return &ConsensusWeights{
		consensusWeights: ads.NewMap[identity.ID, models.TimedBalance](store),
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
