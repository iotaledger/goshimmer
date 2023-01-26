package benchmarks

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types"
)

type ExpectedLikedConflict func(executionConflictAlias string, actualConflictID utxo.TransactionID, actualConflictMembers *set.AdvancedSet[utxo.TransactionID])

type Scenario map[string]*ConflictMeta

func (s *Scenario) ConflictIDB(alias string) utxo.TransactionID {
	return (*s)[alias].ConflictID
}

type ConflictMeta struct {
	Order           int
	ConflictID      utxo.TransactionID
	ParentConflicts *set.AdvancedSet[utxo.TransactionID]
	Conflicting     *set.AdvancedSet[utxo.OutputID]
	ApprovalWeight  int64
}

func newConflictID() (conflictID utxo.OutputID) {
	if err := conflictID.FromRandomness(); err != nil {
		panic(err)
	}
	return conflictID
}
func conflictAlias(pre string, i int) string {
	return fmt.Sprintf("%s_%d", pre, i)
}

var dynamicWeights = make(map[utxo.TransactionID]int64)

var dynamicScenario = func(dsSpent int) (scenarios []Scenario, weightFunc func(utxo.TransactionID) int64) {
	for i := 0; i < dsSpent; i++ {
		conflictID := newConflictID()
		idnA := types.Identifier{}
		idnB := types.Identifier{}
		_ = idnA.FromRandomness()
		_ = idnB.FromRandomness()
		conflictIDA := utxo.TransactionID{Identifier: idnA}
		conflictIDB := utxo.TransactionID{Identifier: idnB}
		dynamicWeights[conflictIDA] = 6
		dynamicWeights[conflictIDB] = 3
		scenarios = append(scenarios, Scenario{
			conflictAlias("A", i): {
				ConflictID:      conflictIDA,
				ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
				Conflicting:     set.NewAdvancedSet(conflictID),
				ApprovalWeight:  dynamicWeights[conflictIDA],
			},
			conflictAlias("B", i): {
				ConflictID:      conflictIDB,
				ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
				Conflicting:     set.NewAdvancedSet(conflictID),
				ApprovalWeight:  dynamicWeights[conflictIDB],
			},
		})
	}
	return scenarios, func(conflictID utxo.TransactionID) int64 {
		return dynamicWeights[conflictID]
	}
}
