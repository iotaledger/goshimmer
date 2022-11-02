package permanent

import (
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type UnspentOutputIDs struct {
	unspentOutputIDs *ads.Set[utxo.OutputID]
}

func NewUnspentOutputIDs(store kvstore.KVStore) (newUnspentOutputIDs *UnspentOutputIDs) {
	return &UnspentOutputIDs{
		unspentOutputIDs: ads.NewSet[utxo.OutputID](store),
	}
}

func (u *UnspentOutputIDs) Import(outputIDs []utxo.OutputID) {
	for _, outputID := range outputIDs {
		u.Store(outputID)
	}
}

func (u *UnspentOutputIDs) Store(id utxo.OutputID) {
	u.unspentOutputIDs.Add(id)
}

func (u *UnspentOutputIDs) Delete(id utxo.OutputID) (deleted bool) {
	return u.unspentOutputIDs.Delete(id)
}

func (u *UnspentOutputIDs) Root() (root types.Identifier) {
	return u.unspentOutputIDs.Root()
}
