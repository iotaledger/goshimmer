package conflict_test

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
)

type ConflictSet = *conflict.Set[utxo.OutputID, utxo.OutputID]

type ConflictSets = []ConflictSet

var NewConflictSet = conflict.NewSet[utxo.OutputID, utxo.OutputID]
