package conflict_test

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	. "github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

func TestConflictSets(t *testing.T) {
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		pendingTasksCounter := syncutils.NewCounter()

		red := NewSet[utxo.OutputID, utxo.OutputID](outputID("red"))
		blue := NewSet[utxo.OutputID, utxo.OutputID](outputID("blue"))
		green := NewSet[utxo.OutputID, utxo.OutputID](outputID("green"))
		yellow := NewSet[utxo.OutputID, utxo.OutputID](outputID("yellow"))
		fmt.Println("adding A...")
		conflictA := New[utxo.OutputID, utxo.OutputID](
			outputID("A"),
			nil,
			advancedset.New(red),
			weight.New().AddCumulativeWeight(7).SetAcceptanceState(acceptance.Pending),
			pendingTasksCounter,
		)
		fmt.Println("adding B...")
		conflictB := New[utxo.OutputID, utxo.OutputID](
			outputID("B"),
			nil,
			advancedset.New(red, blue),
			weight.New().AddCumulativeWeight(3).SetAcceptanceState(acceptance.Pending),
			pendingTasksCounter,
		)
		fmt.Println("adding C...")
		conflictC := New[utxo.OutputID, utxo.OutputID](
			outputID("C"),
			nil,
			advancedset.New(blue, green),
			weight.New().AddCumulativeWeight(5).SetAcceptanceState(acceptance.Pending),
			pendingTasksCounter,
		)

		fmt.Println("adding D...")
		conflictD := New[utxo.OutputID, utxo.OutputID](
			outputID("D"),
			nil,
			advancedset.New(green, yellow),
			weight.New().AddCumulativeWeight(7).SetAcceptanceState(acceptance.Pending),
			pendingTasksCounter,
		)

		fmt.Println("adding E...")

		conflictE := New[utxo.OutputID, utxo.OutputID](
			outputID("E"),
			nil,
			advancedset.New(yellow),
			weight.New().AddCumulativeWeight(9).SetAcceptanceState(acceptance.Pending),
			pendingTasksCounter,
		)

		preferredInsteadMap := map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictA: conflictA,
			conflictB: conflictA,
			conflictC: conflictC,
			conflictD: conflictE,
			conflictE: conflictE,
		}

		pendingTasksCounter.WaitIsZero()
		assertPreferredInstead(t, preferredInsteadMap)

		fmt.Println("set weight D=10...")

		conflictD.Weight().SetCumulativeWeight(10)
		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictC: conflictD,
			conflictD: conflictD,
			conflictE: conflictD,
		}))

		fmt.Println("set weight D=0...")

		conflictD.Weight().SetCumulativeWeight(0)
		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictC: conflictC,
			conflictD: conflictE,
			conflictE: conflictE,
		}))

		fmt.Println("set weight C=8...")

		conflictC.Weight().SetCumulativeWeight(8)
		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictB: conflictC,
		}))

		fmt.Println("set weight C=8...")

		conflictC.Weight().SetCumulativeWeight(8)
		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictB: conflictC,
		}))

		fmt.Println("set weight D=3...")

		conflictD.Weight().SetCumulativeWeight(3)
		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, preferredInsteadMap)

		fmt.Println("set weight E=1...")

		conflictE.Weight().SetCumulativeWeight(1)
		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictD: conflictC,
		}))

		fmt.Println("set weight E=9...")

		conflictE.Weight().SetCumulativeWeight(9)
		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictD: conflictE,
		}))

		fmt.Println("adding F...")

		conflictF := New[utxo.OutputID, utxo.OutputID](
			outputID("F"),
			nil,
			advancedset.New(yellow),
			weight.New().AddCumulativeWeight(19).SetAcceptanceState(acceptance.Pending),
			pendingTasksCounter,
		)

		pendingTasksCounter.WaitIsZero()

		assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
			conflictD: conflictF,
			conflictE: conflictF,
			conflictF: conflictF,
		}))

		assertCorrectOrder(t, conflictA, conflictB, conflictC, conflictD, conflictE, conflictF)
	}
}

func TestConflictParallel(t *testing.T) {
	sequentialPendingTasksCounter := syncutils.NewCounter()
	parallelPendingTasksCounter := syncutils.NewCounter()

	sequentialConflicts := createConflicts(sequentialPendingTasksCounter)
	sequentialPendingTasksCounter.WaitIsZero()

	parallelConflicts := createConflicts(parallelPendingTasksCounter)
	parallelPendingTasksCounter.WaitIsZero()

	const updateCount = 100000

	permutations := make([]func(conflict *Conflict[utxo.OutputID, utxo.OutputID]), 0)
	for i := 0; i < updateCount; i++ {
		permutations = append(permutations, generateRandomConflictPermutation())
	}

	var wg sync.WaitGroup
	for _, permutation := range permutations {
		targetAlias := lo.Keys(parallelConflicts)[rand.Intn(len(parallelConflicts))]

		permutation(sequentialConflicts[targetAlias])

		wg.Add(1)
		go func(permutation func(conflict *Conflict[utxo.OutputID, utxo.OutputID])) {
			permutation(parallelConflicts[targetAlias])

			wg.Done()
		}(permutation)
	}

	sequentialPendingTasksCounter.WaitIsZero()

	wg.Wait()

	parallelPendingTasksCounter.WaitIsZero()

	lo.ForEach(lo.Keys(parallelConflicts), func(conflictAlias string) {
		assert.EqualValuesf(t, sequentialConflicts[conflictAlias].PreferredInstead().ID(), parallelConflicts[conflictAlias].PreferredInstead().ID(), "parallel conflict %s prefers %s, but sequential conflict prefers %s", conflictAlias, parallelConflicts[conflictAlias].PreferredInstead().ID(), sequentialConflicts[conflictAlias].PreferredInstead().ID())
	})

	assertCorrectOrder(t, lo.Values(sequentialConflicts)...)
	assertCorrectOrder(t, lo.Values(parallelConflicts)...)
}

func TestLikedInstead1(t *testing.T) {
	pendingTasksCounter := syncutils.NewCounter()

	masterBranch := New[utxo.OutputID, utxo.OutputID](outputID("M"), nil, nil, weight.New().SetAcceptanceState(acceptance.Accepted), pendingTasksCounter)
	require.True(t, masterBranch.IsLiked())
	require.True(t, masterBranch.LikedInstead().IsEmpty())

	conflictSet1 := NewSet[utxo.OutputID, utxo.OutputID](outputID("O1"))

	conflict1 := New[utxo.OutputID, utxo.OutputID](outputID("TxA"), advancedset.New(masterBranch), advancedset.New(conflictSet1), weight.New().SetCumulativeWeight(6), pendingTasksCounter)
	conflict2 := New[utxo.OutputID, utxo.OutputID](outputID("TxB"), advancedset.New(masterBranch), advancedset.New(conflictSet1), weight.New().SetCumulativeWeight(3), pendingTasksCounter)

	require.True(t, conflict1.IsPreferred())
	require.True(t, conflict1.IsLiked())
	require.True(t, conflict1.LikedInstead().Size() == 0)

	require.False(t, conflict2.IsPreferred())
	require.False(t, conflict2.IsLiked())
	require.True(t, conflict2.LikedInstead().Size() == 1)
	require.True(t, conflict2.LikedInstead().Has(conflict1))
}

func TestLikedInsteadFromPreferredInstead(t *testing.T) {
	pendingTasksCounter := syncutils.NewCounter()

	masterBranch := New[utxo.OutputID, utxo.OutputID](outputID("M"), nil, nil, weight.New().SetAcceptanceState(acceptance.Accepted), pendingTasksCounter)
	require.True(t, masterBranch.IsLiked())
	require.True(t, masterBranch.LikedInstead().IsEmpty())

	conflictSet1 := NewSet[utxo.OutputID, utxo.OutputID](outputID("O1"))
	conflictA := New[utxo.OutputID, utxo.OutputID](outputID("TxA"), advancedset.New(masterBranch), advancedset.New(conflictSet1), weight.New().SetCumulativeWeight(200), pendingTasksCounter)
	conflictB := New[utxo.OutputID, utxo.OutputID](outputID("TxB"), advancedset.New(masterBranch), advancedset.New(conflictSet1), weight.New().SetCumulativeWeight(100), pendingTasksCounter)

	require.True(t, conflictA.IsPreferred())
	require.True(t, conflictA.IsLiked())
	require.True(t, conflictA.LikedInstead().Size() == 0)

	require.False(t, conflictB.IsPreferred())
	require.False(t, conflictB.IsLiked())
	require.True(t, conflictB.LikedInstead().Size() == 1)
	require.True(t, conflictB.LikedInstead().Has(conflictA))

	conflictSet2 := NewSet[utxo.OutputID, utxo.OutputID](outputID("O2"))
	conflictC := New[utxo.OutputID, utxo.OutputID](outputID("TxC"), advancedset.New(conflictA), advancedset.New(conflictSet2), weight.New().SetCumulativeWeight(200), pendingTasksCounter)
	conflictD := New[utxo.OutputID, utxo.OutputID](outputID("TxD"), advancedset.New(conflictA), advancedset.New(conflictSet2), weight.New().SetCumulativeWeight(100), pendingTasksCounter)

	require.True(t, conflictC.IsPreferred())
	require.True(t, conflictC.IsLiked())
	require.True(t, conflictC.LikedInstead().Size() == 0)

	require.False(t, conflictD.IsPreferred())
	require.False(t, conflictD.IsLiked())
	require.True(t, conflictD.LikedInstead().Size() == 1)
	require.True(t, conflictD.LikedInstead().Has(conflictC))

	conflictB.Weight().SetCumulativeWeight(300)
	pendingTasksCounter.WaitIsZero()

	require.True(t, conflictB.IsPreferred())
	require.True(t, conflictB.IsLiked())
	require.True(t, conflictB.LikedInstead().Size() == 0)

	require.False(t, conflictA.IsPreferred())
	require.False(t, conflictA.IsLiked())
	require.True(t, conflictA.LikedInstead().Size() == 1)
	require.True(t, conflictA.LikedInstead().Has(conflictB))

	require.False(t, conflictD.IsPreferred())
	require.False(t, conflictD.IsLiked())
	require.True(t, conflictD.LikedInstead().Size() == 1)
	require.True(t, conflictD.LikedInstead().Has(conflictB))

	conflictB.Weight().SetCumulativeWeight(100)
	pendingTasksCounter.WaitIsZero()

	require.True(t, conflictA.IsPreferred())
	require.True(t, conflictA.IsLiked())
	require.True(t, conflictA.LikedInstead().Size() == 0)

	require.False(t, conflictB.IsPreferred())
	require.False(t, conflictB.IsLiked())
	require.True(t, conflictB.LikedInstead().Size() == 1)
	require.True(t, conflictB.LikedInstead().Has(conflictA))

	require.True(t, conflictC.IsPreferred())
	require.True(t, conflictC.IsLiked())
	require.True(t, conflictC.LikedInstead().Size() == 0)

	require.False(t, conflictD.IsPreferred())
	require.False(t, conflictD.IsLiked())
	require.Equal(t, 1, conflictD.LikedInstead().Size())
	require.True(t, conflictD.LikedInstead().Has(conflictC))

}

func TestLikedInstead21(t *testing.T) {
	pendingTasksCounter := syncutils.NewCounter()

	masterBranch := New[utxo.OutputID, utxo.OutputID](outputID("M"), nil, nil, weight.New().SetAcceptanceState(acceptance.Accepted), pendingTasksCounter)
	require.True(t, masterBranch.IsLiked())
	require.True(t, masterBranch.LikedInstead().IsEmpty())

	conflictSet1 := NewSet[utxo.OutputID, utxo.OutputID](outputID("O1"))
	conflictA := New[utxo.OutputID, utxo.OutputID](outputID("TxA"), advancedset.New(masterBranch), advancedset.New(conflictSet1), weight.New().SetCumulativeWeight(200), pendingTasksCounter)
	conflictB := New[utxo.OutputID, utxo.OutputID](outputID("TxB"), advancedset.New(masterBranch), advancedset.New(conflictSet1), weight.New().SetCumulativeWeight(100), pendingTasksCounter)

	require.True(t, conflictA.IsPreferred())
	require.True(t, conflictA.IsLiked())
	require.True(t, conflictA.LikedInstead().Size() == 0)

	require.False(t, conflictB.IsPreferred())
	require.False(t, conflictB.IsLiked())
	require.True(t, conflictB.LikedInstead().Size() == 1)
	require.True(t, conflictB.LikedInstead().Has(conflictA))

	conflictSet4 := NewSet[utxo.OutputID, utxo.OutputID](outputID("O4"))
	conflictF := New[utxo.OutputID, utxo.OutputID](outputID("TxF"), advancedset.New(conflictA), advancedset.New(conflictSet4), weight.New().SetCumulativeWeight(20), pendingTasksCounter)
	conflictG := New[utxo.OutputID, utxo.OutputID](outputID("TxG"), advancedset.New(conflictA), advancedset.New(conflictSet4), weight.New().SetCumulativeWeight(10), pendingTasksCounter)

	require.True(t, conflictF.IsPreferred())
	require.True(t, conflictF.IsLiked())
	require.True(t, conflictF.LikedInstead().Size() == 0)

	require.False(t, conflictG.IsPreferred())
	require.False(t, conflictG.IsLiked())
	require.True(t, conflictG.LikedInstead().Size() == 1)
	require.True(t, conflictG.LikedInstead().Has(conflictF))

	conflictSet2 := NewSet[utxo.OutputID, utxo.OutputID](outputID("O2"))
	conflictC := New[utxo.OutputID, utxo.OutputID](outputID("TxC"), advancedset.New(masterBranch), advancedset.New(conflictSet2), weight.New().SetCumulativeWeight(200), pendingTasksCounter)
	conflictH := New[utxo.OutputID, utxo.OutputID](outputID("TxH"), advancedset.New(masterBranch, conflictA), advancedset.New(conflictSet2, conflictSet4), weight.New().SetCumulativeWeight(150), pendingTasksCounter)

	require.True(t, conflictC.IsPreferred())
	require.True(t, conflictC.IsLiked())
	require.True(t, conflictC.LikedInstead().Size() == 0)

	require.False(t, conflictH.IsPreferred())
	require.False(t, conflictH.IsLiked())
	require.True(t, conflictH.LikedInstead().Size() == 1)
	require.True(t, conflictH.LikedInstead().Has(conflictC))

	conflictSet3 := NewSet[utxo.OutputID, utxo.OutputID](outputID("O12"))
	conflictI := New[utxo.OutputID, utxo.OutputID](outputID("TxI"), advancedset.New(conflictF), advancedset.New(conflictSet3), weight.New().SetCumulativeWeight(5), pendingTasksCounter)
	conflictJ := New[utxo.OutputID, utxo.OutputID](outputID("TxJ"), advancedset.New(conflictF), advancedset.New(conflictSet3), weight.New().SetCumulativeWeight(15), pendingTasksCounter)

	require.True(t, conflictJ.IsPreferred())
	require.True(t, conflictJ.IsLiked())
	require.True(t, conflictJ.LikedInstead().Size() == 0)

	require.False(t, conflictI.IsPreferred())
	require.False(t, conflictI.IsLiked())
	require.True(t, conflictI.LikedInstead().Size() == 1)
	require.True(t, conflictI.LikedInstead().Has(conflictJ))

	conflictH.Weight().SetCumulativeWeight(250)

	pendingTasksCounter.WaitIsZero()

	require.True(t, conflictH.IsPreferred())
	require.True(t, conflictH.IsLiked())
	require.True(t, conflictH.LikedInstead().Size() == 0)

	require.False(t, conflictF.IsPreferred())
	require.False(t, conflictF.IsLiked())
	require.True(t, conflictF.LikedInstead().Size() == 1)
	require.True(t, conflictF.LikedInstead().Has(conflictH))

	require.False(t, conflictG.IsPreferred())
	require.False(t, conflictG.IsLiked())
	require.True(t, conflictG.LikedInstead().Size() == 1)
	require.True(t, conflictG.LikedInstead().Has(conflictH))

	require.True(t, conflictJ.IsPreferred())
	require.False(t, conflictJ.IsLiked())
	require.True(t, conflictJ.LikedInstead().Size() == 1)
	require.True(t, conflictJ.LikedInstead().Has(conflictH))
}

func assertCorrectOrder(t *testing.T, conflicts ...*Conflict[utxo.OutputID, utxo.OutputID]) {
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Compare(conflicts[j]) == weight.Heavier
	})

	preferredConflicts := advancedset.New[*Conflict[utxo.OutputID, utxo.OutputID]]()
	unPreferredConflicts := advancedset.New[*Conflict[utxo.OutputID, utxo.OutputID]]()

	for _, conflict := range conflicts {
		if !unPreferredConflicts.Has(conflict) {
			preferredConflicts.Add(conflict)
			_ = conflict.ForEachConflictingConflict(func(conflictingConflict *Conflict[utxo.OutputID, utxo.OutputID]) error {
				unPreferredConflicts.Add(conflictingConflict)
				return nil
			})
		}
	}

	for _, conflict := range conflicts {
		if preferredConflicts.Has(conflict) {
			require.True(t, conflict.IsPreferred(), "conflict %s should be preferred", conflict.ID())
		}
		if unPreferredConflicts.Has(conflict) {
			require.False(t, conflict.IsPreferred(), "conflict %s should be unPreferred", conflict.ID())
		}
	}

	_ = unPreferredConflicts.ForEach(func(unPreferredConflict *Conflict[utxo.OutputID, utxo.OutputID]) (err error) {
		// iterating in descending order, so the first preferred conflict
		_ = unPreferredConflict.ForEachConflictingConflict(func(conflictingConflict *Conflict[utxo.OutputID, utxo.OutputID]) error {
			if conflictingConflict.IsPreferred() {
				require.Equal(t, conflictingConflict, unPreferredConflict.PreferredInstead())
				return errors.New("break the loop")
			}
			return nil
		})
		return nil
	})
}

func generateRandomConflictPermutation() func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
	updateType := rand.Intn(100)
	delta := rand.Intn(100)

	return func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
		if updateType%2 == 0 {
			conflict.Weight().AddCumulativeWeight(int64(delta))
		} else {
			conflict.Weight().RemoveCumulativeWeight(int64(delta))
		}
	}
}

func createConflicts(pendingTasksCounter *syncutils.Counter) map[string]*Conflict[utxo.OutputID, utxo.OutputID] {
	red := NewSet[utxo.OutputID, utxo.OutputID](outputID("red"))
	blue := NewSet[utxo.OutputID, utxo.OutputID](outputID("blue"))
	green := NewSet[utxo.OutputID, utxo.OutputID](outputID("green"))
	yellow := NewSet[utxo.OutputID, utxo.OutputID](outputID("yellow"))
	fmt.Println("adding A...")
	conflictA := New[utxo.OutputID, utxo.OutputID](
		outputID("A"),
		nil,
		advancedset.New(red),
		weight.New().SetAcceptanceState(acceptance.Pending),
		pendingTasksCounter,
	)
	fmt.Println("adding B...")
	conflictB := New[utxo.OutputID, utxo.OutputID](
		outputID("B"),
		nil,
		advancedset.New(red, blue),
		weight.New().SetAcceptanceState(acceptance.Pending),
		pendingTasksCounter,
	)
	fmt.Println("adding C...")
	conflictC := New[utxo.OutputID, utxo.OutputID](
		outputID("C"),
		nil,
		advancedset.New(green, blue),
		weight.New().SetAcceptanceState(acceptance.Pending),
		pendingTasksCounter,
	)

	fmt.Println("adding D...")
	conflictD := New[utxo.OutputID, utxo.OutputID](
		outputID("D"),
		nil,
		advancedset.New(green, yellow),
		weight.New().SetAcceptanceState(acceptance.Pending),
		pendingTasksCounter,
	)

	fmt.Println("adding E...")

	conflictE := New[utxo.OutputID, utxo.OutputID](
		outputID("E"),
		nil,
		advancedset.New(yellow),
		weight.New().SetAcceptanceState(acceptance.Pending),
		pendingTasksCounter,
	)

	return map[string]*Conflict[utxo.OutputID, utxo.OutputID]{
		"conflictA": conflictA,
		"conflictB": conflictB,
		"conflictC": conflictC,
		"conflictD": conflictD,
		"conflictE": conflictE,
	}
}

func assertPreferredInstead(t *testing.T, preferredInsteadMap map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]) {
	for conflict, preferredInsteadConflict := range preferredInsteadMap {
		assert.Equalf(t, preferredInsteadConflict.ID(), conflict.PreferredInstead().ID(), "conflict %s should prefer %s instead of %s", conflict.ID(), preferredInsteadConflict.ID(), conflict.PreferredInstead().ID())
	}
}
