package permanent

import (
	"github.com/iotaledger/hive.go/core/kvstore"
)

type UnspentOutputs struct {
	kvstore.KVStore
}

func NewUnspentOutputs(store kvstore.KVStore) (newUnspentOutputs *UnspentOutputs) {
	return &UnspentOutputs{store}
}
