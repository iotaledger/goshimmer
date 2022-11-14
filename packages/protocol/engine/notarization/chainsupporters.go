package notarization

import (
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/identity"
)

type ChainSupporters struct{
	supporters *memstorage.EpochStorage[identity.ID, *memstorage.EpochStorage[models.BlockID, *WeightProof]]
	
}

 /*
	//map[epoch.Index]map[identity.ID]map[commitment.ID]map[BlockID]weightProof
	type weightProof struct {
	signature []byte
	timestamp time.Time
	}

map[epoch.Index]map[identity.ID]map[commitment.ID]map[BlockID]weightProof
map[epoch.Index]map[identity.ID]BlockIDs

*/