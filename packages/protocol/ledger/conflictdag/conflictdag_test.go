package conflictdag

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/hive.go/lo"
)

func TestConflictDAG_CreateConflict(t *testing.T) {
	tf := NewDefaultTestFramework(t)

	tf.CreateConflict("A", tf.ConflictIDs(), "1")
	tf.CreateConflict("B", tf.ConflictIDs(), "1", "2")
	tf.CreateConflict("C", tf.ConflictIDs(), "2")
	tf.CreateConflict("H", tf.ConflictIDs("A"), "2", "4")
	tf.CreateConflict("F", tf.ConflictIDs("A"), "4")
	tf.CreateConflict("G", tf.ConflictIDs("A"), "4")
	tf.CreateConflict("I", tf.ConflictIDs("H"), "14")
	tf.CreateConflict("J", tf.ConflictIDs("H"), "14")
	tf.CreateConflict("K", tf.ConflictIDs(), "17")
	tf.CreateConflict("L", tf.ConflictIDs(), "17")
	tf.CreateConflict("M", tf.ConflictIDs("L"), "19")
	tf.CreateConflict("N", tf.ConflictIDs("L"), "19")
	tf.CreateConflict("O", tf.ConflictIDs("H", "L"), "14", "19")

	tf.AssertConflictSetsAndConflicts(map[string][]string{
		"1":  {"A", "B"},
		"2":  {"B", "C", "H"},
		"4":  {"H", "F", "G"},
		"14": {"I", "J", "O"},
		"17": {"K", "L"},
		"19": {"M", "N", "O"},
	})
	tf.AssertConflictParentsAndChildren(map[string][]string{
		"A": {},
		"B": {},
		"C": {},
		"H": {"A"},
		"F": {"A"},
		"G": {"A"},
		"I": {"H"},
		"J": {"H"},
		"K": {},
		"L": {},
		"M": {"L"},
		"N": {"L"},
		"O": {"H", "L"},
	})

	confirmationState := make(map[string]confirmation.State)
	tf.AssertConfirmationState(lo.MergeMaps(confirmationState, map[string]confirmation.State{
		"A": confirmation.Pending,
		"B": confirmation.Pending,
		"C": confirmation.Pending,
		"H": confirmation.Pending,
		"F": confirmation.Pending,
		"G": confirmation.Pending,
		"I": confirmation.Pending,
		"J": confirmation.Pending,
		"K": confirmation.Pending,
		"L": confirmation.Pending,
		"M": confirmation.Pending,
		"N": confirmation.Pending,
		"O": confirmation.Pending,
	}))

	tf.SetConflictAccepted("H")
	tf.AssertConfirmationState(lo.MergeMaps(confirmationState, map[string]confirmation.State{
		"A": confirmation.Accepted,
		"B": confirmation.Rejected,
		"C": confirmation.Rejected,
		"H": confirmation.Accepted,
		"F": confirmation.Rejected,
		"G": confirmation.Rejected,
	}))

	tf.SetConflictAccepted("K")
	tf.AssertConfirmationState(lo.MergeMaps(confirmationState, map[string]confirmation.State{
		"K": confirmation.Accepted,
		"L": confirmation.Rejected,
		"M": confirmation.Rejected,
		"N": confirmation.Rejected,
		"O": confirmation.Rejected,
	}))

	tf.SetConflictAccepted("I")
	tf.AssertConfirmationState(lo.MergeMaps(confirmationState, map[string]confirmation.State{
		"I": confirmation.Accepted,
		"J": confirmation.Rejected,
	}))
}

func TestConflictDAG_UpdateConflictParents(t *testing.T) {
	tf := NewDefaultTestFramework(t)

	tf.CreateConflict("A", tf.ConflictIDs(), "1")
	tf.CreateConflict("B", tf.ConflictIDs(), "1", "2")
	tf.CreateConflict("C", tf.ConflictIDs(), "2")
	tf.CreateConflict("H", tf.ConflictIDs("A"), "2", "4")
	tf.CreateConflict("F", tf.ConflictIDs("A"), "4")
	tf.CreateConflict("G", tf.ConflictIDs("A"), "4")
	tf.CreateConflict("I", tf.ConflictIDs("H"), "14")
	tf.CreateConflict("J", tf.ConflictIDs("H"), "14")
	tf.CreateConflict("K", tf.ConflictIDs(), "17")
	tf.CreateConflict("L", tf.ConflictIDs(), "17")
	tf.CreateConflict("M", tf.ConflictIDs("L"), "19")
	tf.CreateConflict("N", tf.ConflictIDs("L"), "19")
	tf.CreateConflict("O", tf.ConflictIDs("H", "L"), "14", "19")

	tf.AssertConflictSetsAndConflicts(map[string][]string{
		"1":  {"A", "B"},
		"2":  {"B", "C", "H"},
		"4":  {"H", "F", "G"},
		"14": {"I", "J", "O"},
		"17": {"K", "L"},
		"19": {"M", "N", "O"},
	})
	tf.AssertConflictParentsAndChildren(map[string][]string{
		"A": {},
		"B": {},
		"C": {},
		"H": {"A"},
		"F": {"A"},
		"G": {"A"},
		"I": {"H"},
		"J": {"H"},
		"K": {},
		"L": {},
		"M": {"L"},
		"N": {"L"},
		"O": {"H", "L"},
	})

	tf.AssertConfirmationState(map[string]confirmation.State{
		"A": confirmation.Pending,
		"B": confirmation.Pending,
		"C": confirmation.Pending,
		"H": confirmation.Pending,
		"F": confirmation.Pending,
		"G": confirmation.Pending,
		"I": confirmation.Pending,
		"J": confirmation.Pending,
		"K": confirmation.Pending,
		"L": confirmation.Pending,
		"M": confirmation.Pending,
		"N": confirmation.Pending,
		"O": confirmation.Pending,
	})

	tf.UpdateConflictingResources("G", "14")
	tf.AssertConflictSetsAndConflicts(map[string][]string{
		"1":  {"A", "B"},
		"2":  {"B", "C", "H"},
		"4":  {"H", "F", "G"},
		"14": {"I", "J", "O", "G"},
		"17": {"K", "L"},
		"19": {"M", "N", "O"},
	})

	tf.UpdateConflictParents("O", "K", "H", "L")
	tf.AssertConflictParentsAndChildren(map[string][]string{
		"A": {},
		"B": {},
		"C": {},
		"H": {"A"},
		"F": {"A"},
		"G": {"A"},
		"I": {"H"},
		"J": {"H"},
		"K": {},
		"L": {},
		"M": {"L"},
		"N": {"L"},
		"O": {"K"},
	})
}

func TestConflictDAG_SetNotConflicting_1(t *testing.T) {
	tf := NewDefaultTestFramework(t)

	tf.CreateConflict("X", tf.ConflictIDs(), "0")
	tf.CreateConflict("Y", tf.ConflictIDs(), "0")
	tf.CreateConflict("A", tf.ConflictIDs("X"), "1")
	tf.CreateConflict("B", tf.ConflictIDs("X"), "1")
	tf.CreateConflict("C", tf.ConflictIDs("A"), "2")
	tf.CreateConflict("D", tf.ConflictIDs("A"), "2")
	tf.CreateConflict("E", tf.ConflictIDs("B"), "3")
	tf.CreateConflict("F", tf.ConflictIDs("B"), "3")

	tf.AssertConflictParentsAndChildren(map[string][]string{
		"X": {},
		"Y": {},
		"A": {"X"},
		"B": {"X"},
		"C": {"A"},
		"D": {"A"},
		"E": {"B"},
		"F": {"B"},
	})

	tf.Instance.HandleOrphanedConflict(tf.ConflictID("B"))

	tf.AssertConfirmationState(map[string]confirmation.State{
		"X": confirmation.Pending,
		"Y": confirmation.Pending,
		"A": confirmation.NotConflicting,
		"B": confirmation.Rejected,
		"C": confirmation.Pending,
		"D": confirmation.Pending,
		"E": confirmation.Rejected,
		"F": confirmation.Rejected,
	})

	tf.SetConflictAccepted("C")

	tf.AssertConfirmationState(map[string]confirmation.State{
		"X": confirmation.Accepted,
		"Y": confirmation.Rejected,
		"A": confirmation.NotConflicting,
		"B": confirmation.Rejected,
		"C": confirmation.Accepted,
		"D": confirmation.Rejected,
		"E": confirmation.Rejected,
		"F": confirmation.Rejected,
	})
}

func TestConflictDAG_SetNotConflicting_2(t *testing.T) {
	tf := NewDefaultTestFramework(t)

	tf.CreateConflict("X", tf.ConflictIDs(), "0")
	tf.CreateConflict("Y", tf.ConflictIDs(), "0")
	tf.CreateConflict("A", tf.ConflictIDs("X"), "1", "2")
	tf.CreateConflict("B", tf.ConflictIDs("X"), "1")
	tf.CreateConflict("C", tf.ConflictIDs(), "2")
	tf.CreateConflict("D", tf.ConflictIDs(), "2")
	tf.CreateConflict("E", tf.ConflictIDs("B"), "3")
	tf.CreateConflict("F", tf.ConflictIDs("B"), "3")

	tf.AssertConflictParentsAndChildren(map[string][]string{
		"X": {},
		"Y": {},
		"A": {"X"},
		"B": {"X"},
		"C": {},
		"D": {},
		"E": {"B"},
		"F": {"B"},
	})

	tf.Instance.HandleOrphanedConflict(tf.ConflictID("B"))

	tf.AssertConfirmationState(map[string]confirmation.State{
		"X": confirmation.Pending,
		"Y": confirmation.Pending,
		"A": confirmation.Pending,
		"B": confirmation.Rejected,
		"C": confirmation.Pending,
		"D": confirmation.Pending,
		"E": confirmation.Rejected,
		"F": confirmation.Rejected,
	})

	tf.SetConflictAccepted("A")

	tf.AssertConfirmationState(map[string]confirmation.State{
		"X": confirmation.Accepted,
		"Y": confirmation.Rejected,
		"A": confirmation.Accepted,
		"B": confirmation.Rejected,
		"C": confirmation.Rejected,
		"D": confirmation.Rejected,
		"E": confirmation.Rejected,
		"F": confirmation.Rejected,
	})
}

func TestConflictDAG_SetNotConflicting_3(t *testing.T) {
	tf := NewDefaultTestFramework(t)

	tf.CreateConflict("X", tf.ConflictIDs(), "0")
	tf.CreateConflict("Y", tf.ConflictIDs(), "0")
	tf.CreateConflict("A", tf.ConflictIDs("X"), "1", "2")
	tf.CreateConflict("B", tf.ConflictIDs("X"), "1")
	tf.CreateConflict("C", tf.ConflictIDs(), "2")
	tf.CreateConflict("D", tf.ConflictIDs(), "2")
	tf.CreateConflict("E", tf.ConflictIDs("B"), "3")
	tf.CreateConflict("F", tf.ConflictIDs("B"), "3")

	tf.AssertConflictParentsAndChildren(map[string][]string{
		"X": {},
		"Y": {},
		"A": {"X"},
		"B": {"X"},
		"C": {},
		"D": {},
		"E": {"B"},
		"F": {"B"},
	})

	tf.Instance.HandleOrphanedConflict(tf.ConflictID("B"))

	tf.AssertConfirmationState(map[string]confirmation.State{
		"X": confirmation.Pending,
		"Y": confirmation.Pending,
		"A": confirmation.Pending,
		"B": confirmation.Rejected,
		"C": confirmation.Pending,
		"D": confirmation.Pending,
		"E": confirmation.Rejected,
		"F": confirmation.Rejected,
	})

	tf.Instance.HandleOrphanedConflict(tf.ConflictID("C"))

	tf.AssertConfirmationState(map[string]confirmation.State{
		"X": confirmation.Pending,
		"Y": confirmation.Pending,
		"A": confirmation.Pending,
		"B": confirmation.Rejected,
		"C": confirmation.Rejected,
		"D": confirmation.Pending,
		"E": confirmation.Rejected,
		"F": confirmation.Rejected,
	})

	tf.Instance.HandleOrphanedConflict(tf.ConflictID("D"))

	tf.AssertConfirmationState(map[string]confirmation.State{
		"X": confirmation.Pending,
		"Y": confirmation.Pending,
		"A": confirmation.NotConflicting,
		"B": confirmation.Rejected,
		"C": confirmation.Rejected,
		"D": confirmation.Rejected,
		"E": confirmation.Rejected,
		"F": confirmation.Rejected,
	})
}
