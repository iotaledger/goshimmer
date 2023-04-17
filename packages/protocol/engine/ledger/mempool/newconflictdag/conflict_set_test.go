package newconflictdag

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
)

type TestConflictSet = *ConflictSet[utxo.OutputID, utxo.OutputID, vote.MockedPower]

var NewTestConflictSet = NewConflictSet[utxo.OutputID, utxo.OutputID, vote.MockedPower]
