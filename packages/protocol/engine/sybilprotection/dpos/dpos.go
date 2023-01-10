package dpos

import (
	"github.com/iotaledger/hive.go/core/identity"

	ledgerModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

func ProcessCreatedOutput(output *ledgerModels.OutputWithMetadata, weightUpdater func(id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		weightUpdater(output.ConsensusManaPledgeID(), int64(iotaBalance))
	}
}

func ProcessSpentOutput(output *ledgerModels.OutputWithMetadata, weightUpdater func(id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		weightUpdater(output.ConsensusManaPledgeID(), -int64(iotaBalance))
	}
}
