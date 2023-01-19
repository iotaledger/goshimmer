package benchmarks

import (
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types"
)

type ExpectedLikedConflict func(executionConflictAlias string, actualConflictID utxo.TransactionID, actualConflictMembers *set.AdvancedSet[utxo.TransactionID])

type Scenario map[string]*ConflictMeta

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

var (
	conflictID0  = newConflictID()
	conflictID1  = newConflictID()
	conflictID2  = newConflictID()
	conflictID3  = newConflictID()
	conflictID4  = newConflictID()
	conflictID5  = newConflictID()
	conflictID6  = newConflictID()
	conflictID7  = newConflictID()
	conflictID8  = newConflictID()
	conflictID9  = newConflictID()
	conflictID11 = newConflictID()
	conflictID12 = newConflictID()

	s1 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  6,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  3,
		},
	}

	s2 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  6,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  8,
		},
	}

	s3 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  5,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  4,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  2,
		},
	}

	s4 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  3,
		},
	}

	s45 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{200}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  3,
		},
	}

	s5 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  1,
		},
	}

	s6 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID5),
			ApprovalWeight:  4,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  2,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID5),
			ApprovalWeight:  1,
		},
	}

	s7 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  10,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  15,
		},
	}

	s8 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  10,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0),
			ApprovalWeight:  50,
		},
	}

	s9 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  10,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0),
			ApprovalWeight:  1,
		},
	}

	s10 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight:  1,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID2),
			ApprovalWeight:  3,
		},
	}

	s12 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  25,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  35,
		},
	}

	s13 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  40,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  35,
		},
	}

	s14 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  40,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  35,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  2,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  17,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  10,
		},
		"I": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  5,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  4,
		},
		"K": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  6,
		},
	}

	s15 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  20,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  35,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  2,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  17,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  10,
		},
		"I": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  5,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  4,
		},
		"K": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  6,
		},
	}

	s16 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  20,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  30,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  20,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  2,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  3,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  15,
		},
	}

	s17 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  30,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  10,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  20,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  2,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  3,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  15,
		},
	}

	s18 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  30,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  10,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  5,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  2,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  3,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  15,
		},
		"K": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  10,
		},
		"L": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  20,
		},
		"M": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  5,
		},
		"N": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{13}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  6,
		},
		"I": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{14}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  7,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{15}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  8,
		},
		"O": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{16}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}, utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID8, conflictID12),
			ApprovalWeight:  5,
		},
	}

	s19 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  30,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  10,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  5,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  2,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  3,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  15,
		},
		"K": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  10,
		},
		"L": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  20,
		},
		"M": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  5,
		},
		"N": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{13}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  6,
		},
		"I": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{14}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  7,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{15}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  8,
		},
		"O": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{16}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}, utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID8, conflictID12),
			ApprovalWeight:  9,
		},
	}

	s20 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  200,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  300,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  200,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  20,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  30,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  150,
		},
		"I": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{7}}),
			Conflicting:     set.NewAdvancedSet(conflictID7),
			ApprovalWeight:  5,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{7}}),
			Conflicting:     set.NewAdvancedSet(conflictID7),
			ApprovalWeight:  15,
		},
	}
)
