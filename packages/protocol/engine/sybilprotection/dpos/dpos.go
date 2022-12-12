package dpos

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

func ApplyCreatedOutput(output *ledger.OutputWithMetadata, weightUpdater func(index epoch.Index, id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		weightUpdater(output.Index(), output.ConsensusManaPledgeID(), int64(iotaBalance))
	}
}

func ApplySpentOutput(output *ledger.OutputWithMetadata, weightUpdater func(index epoch.Index, id identity.ID, diff int64)) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		weightUpdater(output.Index(), output.ConsensusManaPledgeID(), -int64(iotaBalance))
	}
}
