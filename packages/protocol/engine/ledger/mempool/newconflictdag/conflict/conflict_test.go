package conflict

import (
	"errors"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

type TestConflict = *Conflict[utxo.OutputID, utxo.OutputID, vote.MockedPower]

type TestConflicts = []TestConflict

var NewTestConflict = New[utxo.OutputID, utxo.OutputID, vote.MockedPower]

func TestConflict_SetRejected(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	conflict1 := NewTestConflict(id("Conflict1"), nil, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflict2 := NewTestConflict(id("Conflict2"), TestConflicts{conflict1}, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflict3 := NewTestConflict(id("Conflict3"), TestConflicts{conflict2}, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	conflict1.setAcceptanceState(acceptance.Rejected)
	require.True(t, conflict1.IsRejected())
	require.True(t, conflict2.IsRejected())
	require.True(t, conflict3.IsRejected())

	conflict4 := NewTestConflict(id("Conflict4"), TestConflicts{conflict1}, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	require.True(t, conflict4.IsRejected())
}

func TestConflict_UpdateParents(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	conflict1 := NewTestConflict(id("Conflict1"), nil, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflict2 := NewTestConflict(id("Conflict2"), nil, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflict3 := NewTestConflict(id("Conflict3"), TestConflicts{conflict1, conflict2}, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflict3.Parents.Has(conflict1))
	require.True(t, conflict3.Parents.Has(conflict2))
}

func TestConflict_SetAccepted(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	{
		conflictSet1 := NewConflictSet(id("ConflictSet1"))
		conflictSet2 := NewConflictSet(id("ConflictSet2"))

		conflict1 := NewTestConflict(id("Conflict1"), nil, ConflictSets{conflictSet1}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
		conflict2 := NewTestConflict(id("Conflict2"), nil, ConflictSets{conflictSet1, conflictSet2}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
		conflict3 := NewTestConflict(id("Conflict3"), nil, ConflictSets{conflictSet2}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

		conflict1.setAcceptanceState(acceptance.Accepted)
		require.True(t, conflict1.IsAccepted())
		require.True(t, conflict2.IsRejected())
		require.True(t, conflict3.IsPending())
	}

	{
		conflictSet1 := NewConflictSet(id("ConflictSet1"))
		conflictSet2 := NewConflictSet(id("ConflictSet2"))

		conflict1 := NewTestConflict(id("Conflict1"), nil, ConflictSets{conflictSet1}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
		conflict2 := NewTestConflict(id("Conflict2"), nil, ConflictSets{conflictSet1, conflictSet2}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
		conflict3 := NewTestConflict(id("Conflict3"), nil, ConflictSets{conflictSet2}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

		conflict2.setAcceptanceState(acceptance.Accepted)
		require.True(t, conflict1.IsRejected())
		require.True(t, conflict2.IsAccepted())
		require.True(t, conflict3.IsRejected())
	}
}

func TestConflictSets(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	red := NewConflictSet(id("red"))
	blue := NewConflictSet(id("blue"))
	green := NewConflictSet(id("green"))
	yellow := NewConflictSet(id("yellow"))

	conflictA := NewTestConflict(id("A"), nil, ConflictSets{red}, weight.New(weights).AddCumulativeWeight(7), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictB := NewTestConflict(id("B"), nil, ConflictSets{red, blue}, weight.New(weights).AddCumulativeWeight(3), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictC := NewTestConflict(id("C"), nil, ConflictSets{blue, green}, weight.New(weights).AddCumulativeWeight(5), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictD := NewTestConflict(id("D"), nil, ConflictSets{green, yellow}, weight.New(weights).AddCumulativeWeight(7), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictE := NewTestConflict(id("E"), nil, ConflictSets{yellow}, weight.New(weights).AddCumulativeWeight(9), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	preferredInsteadMap := map[TestConflict]TestConflict{
		conflictA: conflictA,
		conflictB: conflictA,
		conflictC: conflictC,
		conflictD: conflictE,
		conflictE: conflictE,
	}

	pendingTasks.WaitIsZero()
	assertPreferredInstead(t, preferredInsteadMap)

	conflictD.Weight.SetCumulativeWeight(10)
	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[TestConflict]TestConflict{
		conflictC: conflictD,
		conflictD: conflictD,
		conflictE: conflictD,
	}))

	conflictD.Weight.SetCumulativeWeight(0)
	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[TestConflict]TestConflict{
		conflictC: conflictC,
		conflictD: conflictE,
		conflictE: conflictE,
	}))

	conflictC.Weight.SetCumulativeWeight(8)
	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[TestConflict]TestConflict{
		conflictB: conflictC,
	}))

	conflictC.Weight.SetCumulativeWeight(8)
	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[TestConflict]TestConflict{
		conflictB: conflictC,
	}))

	conflictD.Weight.SetCumulativeWeight(3)
	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, preferredInsteadMap)

	conflictE.Weight.SetCumulativeWeight(1)
	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[TestConflict]TestConflict{
		conflictD: conflictC,
	}))

	conflictE.Weight.SetCumulativeWeight(9)
	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[TestConflict]TestConflict{
		conflictD: conflictE,
	}))

	conflictF := NewTestConflict(id("F"), nil, ConflictSets{yellow}, weight.New(weights).AddCumulativeWeight(19), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	pendingTasks.WaitIsZero()

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[TestConflict]TestConflict{
		conflictD: conflictF,
		conflictE: conflictF,
		conflictF: conflictF,
	}))

	assertCorrectOrder(t, conflictA, conflictB, conflictC, conflictD, conflictE, conflictF)
}

func TestConflictParallel(t *testing.T) {
	sequentialPendingTasks := syncutils.NewCounter()
	parallelPendingTasks := syncutils.NewCounter()

	sequentialConflicts := createConflicts(sequentialPendingTasks)
	sequentialPendingTasks.WaitIsZero()

	parallelConflicts := createConflicts(parallelPendingTasks)
	parallelPendingTasks.WaitIsZero()

	const updateCount = 100000

	permutations := make([]func(conflict TestConflict), 0)
	for i := 0; i < updateCount; i++ {
		permutations = append(permutations, generateRandomConflictPermutation())
	}

	var wg sync.WaitGroup
	for _, permutation := range permutations {
		targetAlias := lo.Keys(parallelConflicts)[rand.Intn(len(parallelConflicts))]

		permutation(sequentialConflicts[targetAlias])

		wg.Add(1)
		go func(permutation func(conflict TestConflict)) {
			permutation(parallelConflicts[targetAlias])

			wg.Done()
		}(permutation)
	}

	sequentialPendingTasks.WaitIsZero()

	wg.Wait()

	parallelPendingTasks.WaitIsZero()

	lo.ForEach(lo.Keys(parallelConflicts), func(conflictAlias string) {
		assert.EqualValuesf(t, sequentialConflicts[conflictAlias].PreferredInstead().ID, parallelConflicts[conflictAlias].PreferredInstead().ID, "parallel conflict %s prefers %s, but sequential conflict prefers %s", conflictAlias, parallelConflicts[conflictAlias].PreferredInstead().ID, sequentialConflicts[conflictAlias].PreferredInstead().ID)
	})

	assertCorrectOrder(t, lo.Values(sequentialConflicts)...)
	assertCorrectOrder(t, lo.Values(parallelConflicts)...)
}

func TestLikedInstead1(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	masterBranch := NewTestConflict(id("M"), nil, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	require.True(t, masterBranch.IsLiked())
	require.True(t, masterBranch.LikedInstead().IsEmpty())

	conflictSet1 := NewConflictSet(id("O1"))

	conflict1 := NewTestConflict(id("TxA"), TestConflicts{masterBranch}, ConflictSets{conflictSet1}, weight.New(weights).SetCumulativeWeight(6), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflict2 := NewTestConflict(id("TxB"), TestConflicts{masterBranch}, ConflictSets{conflictSet1}, weight.New(weights).SetCumulativeWeight(3), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflict1.IsPreferred())
	require.True(t, conflict1.IsLiked())
	require.Equal(t, 0, conflict1.LikedInstead().Size())

	require.False(t, conflict2.IsPreferred())
	require.False(t, conflict2.IsLiked())
	require.Equal(t, 1, conflict2.LikedInstead().Size())
	require.True(t, conflict2.LikedInstead().Has(conflict1))
}

func TestLikedInsteadFromPreferredInstead(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	masterBranch := NewTestConflict(id("M"), nil, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	require.True(t, masterBranch.IsLiked())
	require.True(t, masterBranch.LikedInstead().IsEmpty())

	conflictSet1 := NewConflictSet(id("O1"))
	conflictA := NewTestConflict(id("TxA"), TestConflicts{masterBranch}, ConflictSets{conflictSet1}, weight.New(weights).SetCumulativeWeight(200), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictB := NewTestConflict(id("TxB"), TestConflicts{masterBranch}, ConflictSets{conflictSet1}, weight.New(weights).SetCumulativeWeight(100), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflictA.IsPreferred())
	require.True(t, conflictA.IsLiked())
	require.Equal(t, 0, conflictA.LikedInstead().Size())

	require.False(t, conflictB.IsPreferred())
	require.False(t, conflictB.IsLiked())
	require.Equal(t, 1, conflictB.LikedInstead().Size())
	require.True(t, conflictB.LikedInstead().Has(conflictA))

	conflictSet2 := NewConflictSet(id("O2"))
	conflictC := NewTestConflict(id("TxC"), TestConflicts{conflictA}, ConflictSets{conflictSet2}, weight.New(weights).SetCumulativeWeight(200), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictD := NewTestConflict(id("TxD"), TestConflicts{conflictA}, ConflictSets{conflictSet2}, weight.New(weights).SetCumulativeWeight(100), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflictC.IsPreferred())
	require.True(t, conflictC.IsLiked())
	require.Equal(t, 0, conflictC.LikedInstead().Size())

	require.False(t, conflictD.IsPreferred())
	require.False(t, conflictD.IsLiked())
	require.Equal(t, 1, conflictD.LikedInstead().Size())
	require.True(t, conflictD.LikedInstead().Has(conflictC))

	conflictB.Weight.SetCumulativeWeight(300)
	pendingTasks.WaitIsZero()

	require.True(t, conflictB.IsPreferred())
	require.True(t, conflictB.IsLiked())
	require.Equal(t, 0, conflictB.LikedInstead().Size())

	require.False(t, conflictA.IsPreferred())
	require.False(t, conflictA.IsLiked())
	require.Equal(t, 1, conflictA.LikedInstead().Size())
	require.True(t, conflictA.LikedInstead().Has(conflictB))

	require.False(t, conflictD.IsPreferred())
	require.False(t, conflictD.IsLiked())
	require.Equal(t, 1, conflictD.LikedInstead().Size())
	require.True(t, conflictD.LikedInstead().Has(conflictB))

	conflictB.Weight.SetCumulativeWeight(100)
	pendingTasks.WaitIsZero()

	require.True(t, conflictA.IsPreferred())
	require.True(t, conflictA.IsLiked())
	require.Equal(t, 0, conflictA.LikedInstead().Size())

	require.False(t, conflictB.IsPreferred())
	require.False(t, conflictB.IsLiked())
	require.Equal(t, 1, conflictB.LikedInstead().Size())
	require.True(t, conflictB.LikedInstead().Has(conflictA))

	require.True(t, conflictC.IsPreferred())
	require.True(t, conflictC.IsLiked())
	require.Equal(t, 0, conflictC.LikedInstead().Size())

	require.False(t, conflictD.IsPreferred())
	require.False(t, conflictD.IsLiked())
	require.Equal(t, 1, conflictD.LikedInstead().Size())
	require.True(t, conflictD.LikedInstead().Has(conflictC))
}

func TestLikedInstead21(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	masterBranch := NewTestConflict(id("M"), nil, nil, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	require.True(t, masterBranch.IsLiked())
	require.True(t, masterBranch.LikedInstead().IsEmpty())

	conflictSet1 := NewConflictSet(id("O1"))
	conflictA := NewTestConflict(id("TxA"), TestConflicts{masterBranch}, ConflictSets{conflictSet1}, weight.New(weights).SetCumulativeWeight(200), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictB := NewTestConflict(id("TxB"), TestConflicts{masterBranch}, ConflictSets{conflictSet1}, weight.New(weights).SetCumulativeWeight(100), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflictA.IsPreferred())
	require.True(t, conflictA.IsLiked())
	require.Equal(t, 0, conflictA.LikedInstead().Size())

	require.False(t, conflictB.IsPreferred())
	require.False(t, conflictB.IsLiked())
	require.Equal(t, 1, conflictB.LikedInstead().Size())
	require.True(t, conflictB.LikedInstead().Has(conflictA))

	conflictSet4 := NewConflictSet(id("O4"))
	conflictF := NewTestConflict(id("TxF"), TestConflicts{conflictA}, ConflictSets{conflictSet4}, weight.New(weights).SetCumulativeWeight(20), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictG := NewTestConflict(id("TxG"), TestConflicts{conflictA}, ConflictSets{conflictSet4}, weight.New(weights).SetCumulativeWeight(10), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflictF.IsPreferred())
	require.True(t, conflictF.IsLiked())
	require.Equal(t, 0, conflictF.LikedInstead().Size())

	require.False(t, conflictG.IsPreferred())
	require.False(t, conflictG.IsLiked())
	require.Equal(t, 1, conflictG.LikedInstead().Size())
	require.True(t, conflictG.LikedInstead().Has(conflictF))

	conflictSet2 := NewConflictSet(id("O2"))
	conflictC := NewTestConflict(id("TxC"), TestConflicts{masterBranch}, ConflictSets{conflictSet2}, weight.New(weights).SetCumulativeWeight(200), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictH := NewTestConflict(id("TxH"), TestConflicts{masterBranch, conflictA}, ConflictSets{conflictSet2, conflictSet4}, weight.New(weights).SetCumulativeWeight(150), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflictC.IsPreferred())
	require.True(t, conflictC.IsLiked())
	require.Equal(t, 0, conflictC.LikedInstead().Size())

	require.False(t, conflictH.IsPreferred())
	require.False(t, conflictH.IsLiked())
	require.Equal(t, 1, conflictH.LikedInstead().Size())
	require.True(t, conflictH.LikedInstead().Has(conflictC))

	conflictSet3 := NewConflictSet(id("O12"))
	conflictI := NewTestConflict(id("TxI"), TestConflicts{conflictF}, ConflictSets{conflictSet3}, weight.New(weights).SetCumulativeWeight(5), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictJ := NewTestConflict(id("TxJ"), TestConflicts{conflictF}, ConflictSets{conflictSet3}, weight.New(weights).SetCumulativeWeight(15), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	require.True(t, conflictJ.IsPreferred())
	require.True(t, conflictJ.IsLiked())
	require.Equal(t, 0, conflictJ.LikedInstead().Size())

	require.False(t, conflictI.IsPreferred())
	require.False(t, conflictI.IsLiked())
	require.Equal(t, 1, conflictI.LikedInstead().Size())
	require.True(t, conflictI.LikedInstead().Has(conflictJ))

	conflictH.Weight.SetCumulativeWeight(250)

	pendingTasks.WaitIsZero()

	require.True(t, conflictH.IsPreferred())
	require.True(t, conflictH.IsLiked())
	require.Equal(t, 0, conflictH.LikedInstead().Size())

	require.False(t, conflictF.IsPreferred())
	require.False(t, conflictF.IsLiked())
	require.Equal(t, 1, conflictF.LikedInstead().Size())
	require.True(t, conflictF.LikedInstead().Has(conflictH))

	require.False(t, conflictG.IsPreferred())
	require.False(t, conflictG.IsLiked())
	require.Equal(t, 1, conflictG.LikedInstead().Size())
	require.True(t, conflictG.LikedInstead().Has(conflictH))

	require.True(t, conflictJ.IsPreferred())
	require.False(t, conflictJ.IsLiked())
	require.Equal(t, 1, conflictJ.LikedInstead().Size())
	require.True(t, conflictJ.LikedInstead().Has(conflictH))
}

func assertCorrectOrder(t *testing.T, conflicts ...TestConflict) {
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Compare(conflicts[j]) == weight.Heavier
	})

	preferredConflicts := advancedset.New[TestConflict]()
	unPreferredConflicts := advancedset.New[TestConflict]()

	for _, conflict := range conflicts {
		if !unPreferredConflicts.Has(conflict) {
			preferredConflicts.Add(conflict)
			_ = conflict.ConflictingConflicts.ForEach(func(conflictingConflict *Conflict[utxo.OutputID, utxo.OutputID, vote.MockedPower]) error {
				if conflict != conflictingConflict {
					unPreferredConflicts.Add(conflictingConflict)
				}

				return nil
			}, true)
		}
	}

	for _, conflict := range conflicts {
		if preferredConflicts.Has(conflict) {
			require.True(t, conflict.IsPreferred(), "conflict %s should be preferred", conflict.ID)
		}
		if unPreferredConflicts.Has(conflict) {
			require.False(t, conflict.IsPreferred(), "conflict %s should be unPreferred", conflict.ID)
		}
	}

	_ = unPreferredConflicts.ForEach(func(unPreferredConflict TestConflict) (err error) {
		// iterating in descending order, so the first preferred conflict
		return unPreferredConflict.ConflictingConflicts.ForEach(func(conflictingConflict TestConflict) error {
			if conflictingConflict != unPreferredConflict && conflictingConflict.IsPreferred() {
				require.Equal(t, conflictingConflict, unPreferredConflict.PreferredInstead())

				return errors.New("break the loop")
			}

			return nil
		}, true)
	})
}

func generateRandomConflictPermutation() func(conflict TestConflict) {
	updateType := rand.Intn(100)
	delta := rand.Intn(100)

	return func(conflict TestConflict) {
		if updateType%2 == 0 {
			conflict.Weight.AddCumulativeWeight(int64(delta))
		} else {
			conflict.Weight.RemoveCumulativeWeight(int64(delta))
		}
	}
}

func createConflicts(pendingTasks *syncutils.Counter) map[string]TestConflict {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())

	red := NewConflictSet(id("red"))
	blue := NewConflictSet(id("blue"))
	green := NewConflictSet(id("green"))
	yellow := NewConflictSet(id("yellow"))

	conflictA := NewTestConflict(id("A"), nil, ConflictSets{red}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictB := NewTestConflict(id("B"), nil, ConflictSets{red, blue}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictC := NewTestConflict(id("C"), nil, ConflictSets{green, blue}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictD := NewTestConflict(id("D"), nil, ConflictSets{green, yellow}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))
	conflictE := NewTestConflict(id("E"), nil, ConflictSets{yellow}, weight.New(weights), pendingTasks, acceptance.ThresholdProvider(weights.TotalWeight))

	return map[string]TestConflict{
		"conflictA": conflictA,
		"conflictB": conflictB,
		"conflictC": conflictC,
		"conflictD": conflictD,
		"conflictE": conflictE,
	}
}

func assertPreferredInstead(t *testing.T, preferredInsteadMap map[TestConflict]TestConflict) {
	for conflict, preferredInsteadConflict := range preferredInsteadMap {
		assert.Equalf(t, preferredInsteadConflict.ID, conflict.PreferredInstead().ID, "conflict %s should prefer %s instead of %s", conflict.ID, preferredInsteadConflict.ID, conflict.PreferredInstead().ID)
	}
}
