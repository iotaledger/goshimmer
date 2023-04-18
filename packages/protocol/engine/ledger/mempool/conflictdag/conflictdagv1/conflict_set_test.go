package conflictdagv1

import (
	"github.com/iotaledger/goshimmer/packages/core/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
)

type TestConflictSet = *ConflictSet[utxo.OutputID, utxo.OutputID, vote.MockedPower]

var NewTestConflictSet = NewConflictSet[utxo.OutputID, utxo.OutputID, vote.MockedPower]
