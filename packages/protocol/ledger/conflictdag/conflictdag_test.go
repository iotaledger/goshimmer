package conflictdag

import (
	"testing"

	"github.com/iotaledger/hive.go/core/types/confirmation"
)

func TestConflictDAG_CreateConflict(t *testing.T) {
	tf := NewTestFramework(t)

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

	tf.AssertConflictSets(map[string][]string{
		"1":  {"A", "B"},
		"2":  {"B", "C", "H"},
		"4":  {"H", "F", "G"},
		"14": {"I", "J", "O"},
		"17": {"K", "L"},
		"19": {"M", "N", "O"},
	})
	tf.AssertConflictsParents(map[string][]string{
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
	tf.AssertConflictsChildren(map[string][]string{
		"A": {"H", "F", "G"},
		"B": {},
		"C": {},
		"H": {"I", "J", "O"},
		"F": {},
		"G": {},
		"I": {},
		"J": {},
		"K": {},
		"L": {"M", "N", "O"},
		"M": {},
		"N": {},
		"O": {},
	})
	tf.AssertConflictsConflictSets(map[string][]string{
		"A": {"1"},
		"B": {"1", "2"},
		"C": {"2"},
		"H": {"2", "4"},
		"F": {"4"},
		"G": {"4"},
		"I": {"14"},
		"J": {"14"},
		"K": {"17"},
		"L": {"17"},
		"M": {"19"},
		"N": {"19"},
		"O": {"14", "19"},
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

	tf.SetConflictAccepted("H")

	tf.AssertConfirmationState(map[string]confirmation.State{
		"A": confirmation.Accepted,
		"B": confirmation.Rejected,
		"C": confirmation.Rejected,
		"H": confirmation.Accepted,
		"F": confirmation.Rejected,
		"G": confirmation.Rejected,
		"I": confirmation.Pending,
		"J": confirmation.Pending,
		"K": confirmation.Pending,
		"L": confirmation.Pending,
		"M": confirmation.Pending,
		"N": confirmation.Pending,
		"O": confirmation.Pending,
	})

	tf.SetConflictAccepted("K")

	tf.AssertConfirmationState(map[string]confirmation.State{
		"A": confirmation.Accepted,
		"B": confirmation.Rejected,
		"C": confirmation.Rejected,
		"H": confirmation.Accepted,
		"F": confirmation.Rejected,
		"G": confirmation.Rejected,
		"I": confirmation.Pending,
		"J": confirmation.Pending,
		"K": confirmation.Accepted,
		"L": confirmation.Rejected,
		"M": confirmation.Rejected,
		"N": confirmation.Rejected,
		"O": confirmation.Rejected,
	})

	tf.SetConflictAccepted("I")

	tf.AssertConfirmationState(map[string]confirmation.State{
		"A": confirmation.Accepted,
		"B": confirmation.Rejected,
		"C": confirmation.Rejected,
		"H": confirmation.Accepted,
		"F": confirmation.Rejected,
		"G": confirmation.Rejected,
		"I": confirmation.Accepted,
		"J": confirmation.Rejected,
		"K": confirmation.Accepted,
		"L": confirmation.Rejected,
		"M": confirmation.Rejected,
		"N": confirmation.Rejected,
		"O": confirmation.Rejected,
	})
}

func TestConflictDAG_UpdateConflictParents(t *testing.T) {
	tf := NewTestFramework(t)

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

	tf.AssertConflictSets(map[string][]string{
		"1":  {"A", "B"},
		"2":  {"B", "C", "H"},
		"4":  {"H", "F", "G"},
		"14": {"I", "J", "O"},
		"17": {"K", "L"},
		"19": {"M", "N", "O"},
	})
	tf.AssertConflictsParents(map[string][]string{
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
	tf.AssertConflictsChildren(map[string][]string{
		"A": {"H", "F", "G"},
		"B": {},
		"C": {},
		"H": {"I", "J", "O"},
		"F": {},
		"G": {},
		"I": {},
		"J": {},
		"K": {},
		"L": {"M", "N", "O"},
		"M": {},
		"N": {},
		"O": {},
	})
	tf.AssertConflictsConflictSets(map[string][]string{
		"A": {"1"},
		"B": {"1", "2"},
		"C": {"2"},
		"H": {"2", "4"},
		"F": {"4"},
		"G": {"4"},
		"I": {"14"},
		"J": {"14"},
		"K": {"17"},
		"L": {"17"},
		"M": {"19"},
		"N": {"19"},
		"O": {"14", "19"},
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

	tf.AssertConflictsConflictSets(map[string][]string{
		"A": {"1"},
		"B": {"1", "2"},
		"C": {"2"},
		"H": {"2", "4"},
		"F": {"4"},
		"G": {"4", "14"},
		"I": {"14"},
		"J": {"14"},
		"K": {"17"},
		"L": {"17"},
		"M": {"19"},
		"N": {"19"},
		"O": {"14", "19"},
	})

	tf.UpdateConflictParents("O", "K", "H", "L")

	tf.AssertConflictsParents(map[string][]string{
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
	tf.AssertConflictsChildren(map[string][]string{
		"A": {"H", "F", "G"},
		"B": {},
		"C": {},
		"H": {"I", "J"},
		"F": {},
		"G": {},
		"I": {},
		"J": {},
		"K": {"O"},
		"L": {"M", "N"},
		"M": {},
		"N": {},
		"O": {},
	})
}
