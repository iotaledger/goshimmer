package conflict

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
)

type ConflictSet = *Set[utxo.OutputID, utxo.OutputID, vote.MockedPower]

type ConflictSets = []ConflictSet

var NewConflictSet = NewSet[utxo.OutputID, utxo.OutputID, vote.MockedPower]
