package conflictdag

import (
	"testing"

	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

func TestConflictDAG_CreateConflict(t *testing.T) {
	tf := NewTestFramework(t)

	tf.CreateConflict("1", "A", utxo.NewTransactionIDs())
	tf.CreateConflict("1", "B", utxo.NewTransactionIDs())
	tf.CreateConflict("2", "C", utxo.NewTransactionIDs())
	tf.UpdateConflictingResources("B", "2")

	tf.AssertConflictSets(map[string][]string{
		"1": {"A", "B"},
		"2": {"B", "C"},
	})
	tf.AssertConflictsParents(map[string][]string{
		"A": {},
		"B": {},
		"C": {},
	})
	tf.AssertConflictsChildren(map[string][]string{
		"A": {},
		"B": {},
		"C": {},
	})
	tf.AssertConflictsConflictSets(map[string][]string{
		"A": {"1"},
		"B": {"1", "2"},
		"C": {"2"},
	})
	tf.AssertConfirmationState(map[string]confirmation.State{
		"A": confirmation.Pending,
		"B": confirmation.Pending,
		"C": confirmation.Pending,
	})
}
