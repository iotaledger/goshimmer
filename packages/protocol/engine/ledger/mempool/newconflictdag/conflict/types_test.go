package conflict_test

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
)

type Conflict = *conflict.Conflict[utxo.OutputID, utxo.OutputID]

var NewConflict = conflict.New[utxo.OutputID, utxo.OutputID]

type Conflicts = []Conflict

type ConflictSet = *conflict.Set[utxo.OutputID, utxo.OutputID]

var NewConflictSet = conflict.NewSet[utxo.OutputID, utxo.OutputID]

type ConflictSets = []ConflictSet

type SortedConflictSet = *conflict.SortedSet[utxo.OutputID, utxo.OutputID]

var NewSortedConflictSet = conflict.NewSortedSet[utxo.OutputID, utxo.OutputID]
