package conflict

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

type SortedConflictSet = *SortedSet[utxo.OutputID, utxo.OutputID, vote.MockedPower]

var NewSortedConflictSet = NewSortedSet[utxo.OutputID, utxo.OutputID, vote.MockedPower]

func TestSortedConflict(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	conflict1 := newConflict("conflict1", weight.New(weights).AddCumulativeWeight(12), pendingTasks)
	conflict1.SetAcceptanceState(acceptance.Rejected)
	conflict2 := newConflict("conflict2", weight.New(weights).AddCumulativeWeight(10), pendingTasks)
	conflict3 := newConflict("conflict3", weight.New(weights).AddCumulativeWeight(1), pendingTasks)
	conflict3.SetAcceptanceState(acceptance.Accepted)
	conflict4 := newConflict("conflict4", weight.New(weights).AddCumulativeWeight(11), pendingTasks)
	conflict4.SetAcceptanceState(acceptance.Rejected)
	conflict5 := newConflict("conflict5", weight.New(weights).AddCumulativeWeight(11), pendingTasks)
	conflict6 := newConflict("conflict6", weight.New(weights).AddCumulativeWeight(2), pendingTasks)
	conflict6.SetAcceptanceState(acceptance.Accepted)

	sortedConflicts := NewSortedConflictSet(conflict1, pendingTasks)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1")

	sortedConflicts.Add(conflict2)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict2", "conflict1")

	sortedConflicts.Add(conflict3)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict3", "conflict2", "conflict1")

	sortedConflicts.Add(conflict4)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict3", "conflict2", "conflict1", "conflict4")

	sortedConflicts.Add(conflict5)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict3", "conflict5", "conflict2", "conflict1", "conflict4")

	sortedConflicts.Add(conflict6)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict5", "conflict2", "conflict1", "conflict4")

	conflict2.Weight.AddCumulativeWeight(3)
	require.Equal(t, int64(13), conflict2.Weight.Value().CumulativeWeight())
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict2", "conflict5", "conflict1", "conflict4")

	conflict2.Weight.RemoveCumulativeWeight(3)
	require.Equal(t, int64(10), conflict2.Weight.Value().CumulativeWeight())
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict5", "conflict2", "conflict1", "conflict4")

	conflict5.Weight.SetAcceptanceState(acceptance.Accepted)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict5", "conflict6", "conflict3", "conflict2", "conflict1", "conflict4")
}

func TestSortedDecreaseHeaviest(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	conflict1 := newConflict("conflict1", weight.New(weights).AddCumulativeWeight(1), pendingTasks)
	conflict1.SetAcceptanceState(acceptance.Accepted)
	conflict2 := newConflict("conflict2", weight.New(weights).AddCumulativeWeight(2), pendingTasks)

	sortedConflicts := NewSortedConflictSet(conflict1, pendingTasks)

	sortedConflicts.Add(conflict1)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1")

	sortedConflicts.Add(conflict2)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1", "conflict2")

	conflict1.Weight.SetAcceptanceState(acceptance.Pending)
	pendingTasks.WaitIsZero()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict2", "conflict1")
}

func TestSortedConflictParallel(t *testing.T) {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	pendingTasks := syncutils.NewCounter()

	const conflictCount = 1000
	const updateCount = 100000

	conflicts := make(map[string]TestConflict)
	parallelConflicts := make(map[string]TestConflict)
	for i := 0; i < conflictCount; i++ {
		alias := "conflict" + strconv.Itoa(i)

		conflicts[alias] = newConflict(alias, weight.New(weights), pendingTasks)
		parallelConflicts[alias] = newConflict(alias, weight.New(weights), pendingTasks)
	}

	sortedConflicts := NewSortedConflictSet(conflicts["conflict0"], pendingTasks)
	sortedParallelConflicts := NewSortedConflictSet(parallelConflicts["conflict0"], pendingTasks)
	sortedParallelConflicts1 := NewSortedConflictSet(parallelConflicts["conflict0"], pendingTasks)

	for i := 0; i < conflictCount; i++ {
		alias := "conflict" + strconv.Itoa(i)

		sortedConflicts.Add(conflicts[alias])
		sortedParallelConflicts.Add(parallelConflicts[alias])
		sortedParallelConflicts1.Add(parallelConflicts[alias])
	}

	originalSortingBefore := sortedConflicts.String()
	parallelSortingBefore := sortedParallelConflicts.String()
	require.Equal(t, originalSortingBefore, parallelSortingBefore)

	permutations := make([]func(conflict TestConflict), 0)
	for i := 0; i < updateCount; i++ {
		permutations = append(permutations, generateRandomWeightPermutation())
	}

	var wg sync.WaitGroup
	for i, permutation := range permutations {
		targetAlias := "conflict" + strconv.Itoa(i%conflictCount)

		permutation(conflicts[targetAlias])

		wg.Add(1)
		go func(permutation func(conflict TestConflict)) {
			permutation(parallelConflicts[targetAlias])

			wg.Done()
		}(permutation)
	}

	pendingTasks.WaitIsZero()
	wg.Wait()
	pendingTasks.WaitIsZero()

	originalSortingAfter := sortedConflicts.String()
	parallelSortingAfter := sortedParallelConflicts.String()
	require.Equal(t, originalSortingAfter, parallelSortingAfter)
	require.NotEqualf(t, originalSortingBefore, originalSortingAfter, "original sorting should have changed")

	pendingTasks.WaitIsZero()

	parallelSortingAfter = sortedParallelConflicts1.String()
	require.Equal(t, originalSortingAfter, parallelSortingAfter)
	require.NotEqualf(t, originalSortingBefore, originalSortingAfter, "original sorting should have changed")
}

func generateRandomWeightPermutation() func(conflict TestConflict) {
	switch rand.Intn(2) {
	case 0:
		return generateRandomCumulativeWeightPermutation(int64(rand.Intn(100)))
	default:
		// return generateRandomConfirmationStatePermutation()
		return func(conflict TestConflict) {

		}
	}
}

func generateRandomCumulativeWeightPermutation(delta int64) func(conflict TestConflict) {
	updateType := rand.Intn(100)

	return func(conflict TestConflict) {
		if updateType%2 == 0 {
			conflict.Weight.AddCumulativeWeight(delta)
		} else {
			conflict.Weight.RemoveCumulativeWeight(delta)
		}

		conflict.Weight.AddCumulativeWeight(delta)
	}
}

func assertSortedConflictsOrder(t *testing.T, sortedConflicts SortedConflictSet, aliases ...string) {
	require.NoError(t, sortedConflicts.ForEach(func(c TestConflict) error {
		currentAlias := aliases[0]
		aliases = aliases[1:]

		require.Equal(t, "OutputID("+currentAlias+")", c.ID.String())

		return nil
	}))

	require.Empty(t, aliases)
}

func newConflict(alias string, weight *weight.Weight, pendingTasksCounter *syncutils.Counter) TestConflict {
	return NewTestConflict(id(alias), nil, nil, weight, pendingTasksCounter)
}

func id(alias string) utxo.OutputID {
	conflictID := utxo.OutputID{
		TransactionID: utxo.TransactionID{Identifier: blake2b.Sum256([]byte(alias))},
		Index:         0,
	}
	conflictID.RegisterAlias(alias)

	return conflictID
}
