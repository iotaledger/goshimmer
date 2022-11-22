package dpos

import (
	"github.com/iotaledger/hive.go/core/identity"

	ledgerModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

func onOutputCreated(output *ledgerModels.OutputWithMetadata, updateWeights func(id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		updateWeights(output.ConsensusManaPledgeID(), int64(iotaBalance))
	}
}

func onOutputSpent(output *ledgerModels.OutputWithMetadata, processDiff func(id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		processDiff(output.ConsensusManaPledgeID(), -int64(iotaBalance))
	}
}
