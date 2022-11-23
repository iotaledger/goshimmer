package dpos

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
)

func ProcessCreatedOutput(output *ledgerstate.OutputWithMetadata, weightUpdater func(id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		weightUpdater(output.ConsensusManaPledgeID(), int64(iotaBalance))
	}
}

func ProcessSpentOutput(output *ledgerstate.OutputWithMetadata, weightUpdater func(id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		weightUpdater(output.ConsensusManaPledgeID(), -int64(iotaBalance))
	}
}
