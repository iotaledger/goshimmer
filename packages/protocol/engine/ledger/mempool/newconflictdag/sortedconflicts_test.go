package newconflictdag_test

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	. "github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	. "github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

func TestSortedConflict(t *testing.T) {
	sortedConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()

	conflict1 := newConflict("conflict1", New().AddCumulativeWeight(12).SetAcceptanceState(acceptance.Rejected))
	conflict2 := newConflict("conflict2", New().AddCumulativeWeight(10))
	conflict3 := newConflict("conflict3", New().AddCumulativeWeight(1).SetAcceptanceState(acceptance.Accepted))
	conflict4 := newConflict("conflict4", New().AddCumulativeWeight(11).SetAcceptanceState(acceptance.Rejected))
	conflict5 := newConflict("conflict5", New().AddCumulativeWeight(11).SetAcceptanceState(acceptance.Pending))
	conflict6 := newConflict("conflict6", New().AddCumulativeWeight(2).SetAcceptanceState(acceptance.Accepted))

	sortedConflicts.Add(conflict1)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1")

	sortedConflicts.Add(conflict2)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict2", "conflict1")

	sortedConflicts.Add(conflict3)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict3", "conflict2", "conflict1")

	sortedConflicts.Add(conflict4)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict3", "conflict2", "conflict1", "conflict4")

	sortedConflicts.Add(conflict5)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict3", "conflict5", "conflict2", "conflict1", "conflict4")

	sortedConflicts.Add(conflict6)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict5", "conflict2", "conflict1", "conflict4")

	conflict2.Weight().AddCumulativeWeight(3)
	require.Equal(t, int64(13), conflict2.Weight().Value().CumulativeWeight())
	sortedConflicts.WaitSortingDone()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict2", "conflict5", "conflict1", "conflict4")

	conflict2.Weight().RemoveCumulativeWeight(3)
	require.Equal(t, int64(10), conflict2.Weight().Value().CumulativeWeight())

	sortedConflicts.WaitSortingDone()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict5", "conflict2", "conflict1", "conflict4")

	conflict5.Weight().SetAcceptanceState(acceptance.Accepted)
	sortedConflicts.WaitSortingDone()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict5", "conflict6", "conflict3", "conflict2", "conflict1", "conflict4")
}

func TestSortedDecreaseHeaviest(t *testing.T) {
	sortedConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()

	conflict1 := newConflict("conflict1", New().AddCumulativeWeight(1).SetAcceptanceState(acceptance.Accepted))
	conflict2 := newConflict("conflict2", New().AddCumulativeWeight(2).SetAcceptanceState(acceptance.Pending))

	sortedConflicts.Add(conflict1)
	sortedConflicts.WaitSortingDone()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1")

	sortedConflicts.Add(conflict2)
	sortedConflicts.WaitSortingDone()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1", "conflict2")

	conflict1.Weight().SetAcceptanceState(acceptance.Pending)
	sortedConflicts.WaitSortingDone()
	assertSortedConflictsOrder(t, sortedConflicts, "conflict2", "conflict1")
}

func TestSortedConflictParallel(t *testing.T) {
	const conflictCount = 1000
	const updateCount = 100000

	sortedConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()
	sortedParallelConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()
	sortedParallelConflicts1 := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()

	conflicts := make(map[string]*Conflict[utxo.OutputID, utxo.OutputID])
	parallelConflicts := make(map[string]*Conflict[utxo.OutputID, utxo.OutputID])

	for i := 0; i < conflictCount; i++ {
		alias := "conflict" + strconv.Itoa(i)

		conflicts[alias] = newConflict(alias, New())
		parallelConflicts[alias] = newConflict(alias, New())

		sortedConflicts.Add(conflicts[alias])
		sortedParallelConflicts.Add(parallelConflicts[alias])
		sortedParallelConflicts1.Add(parallelConflicts[alias])
	}

	originalSortingBefore := sortedConflicts.String()
	parallelSortingBefore := sortedParallelConflicts.String()
	require.Equal(t, originalSortingBefore, parallelSortingBefore)

	permutations := make([]func(conflict *Conflict[utxo.OutputID, utxo.OutputID]), 0)
	for i := 0; i < updateCount; i++ {
		permutations = append(permutations, generateRandomWeightPermutation())
	}

	var wg sync.WaitGroup
	for i, permutation := range permutations {
		targetAlias := "conflict" + strconv.Itoa(i%conflictCount)

		permutation(conflicts[targetAlias])

		wg.Add(1)
		go func(permutation func(conflict *Conflict[utxo.OutputID, utxo.OutputID])) {
			permutation(parallelConflicts[targetAlias])

			wg.Done()
		}(permutation)
	}

	sortedConflicts.WaitSortingDone()

	wg.Wait()

	sortedParallelConflicts.WaitSortingDone()

	originalSortingAfter := sortedConflicts.String()
	parallelSortingAfter := sortedParallelConflicts.String()
	require.Equal(t, originalSortingAfter, parallelSortingAfter)
	require.NotEqualf(t, originalSortingBefore, originalSortingAfter, "original sorting should have changed")

	sortedParallelConflicts1.WaitSortingDone()

	parallelSortingAfter = sortedParallelConflicts1.String()
	require.Equal(t, originalSortingAfter, parallelSortingAfter)
	require.NotEqualf(t, originalSortingBefore, originalSortingAfter, "original sorting should have changed")
}

func generateRandomWeightPermutation() func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
	switch rand.Intn(2) {
	case 0:
		return generateRandomCumulativeWeightPermutation(int64(rand.Intn(100)))
	default:
		// return generateRandomConfirmationStatePermutation()
		return func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {

		}
	}
}

func generateRandomConfirmationStatePermutation() func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
	updateType := rand.Intn(3)

	return func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
		switch updateType {
		case 0:
			conflict.Weight().SetAcceptanceState(acceptance.Pending)
		case 1:
			conflict.Weight().SetAcceptanceState(acceptance.Accepted)
		case 2:
			conflict.Weight().SetAcceptanceState(acceptance.Rejected)
		}
	}
}

func generateRandomCumulativeWeightPermutation(delta int64) func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
	updateType := rand.Intn(100)

	return func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
		if updateType%2 == 0 {
			conflict.Weight().AddCumulativeWeight(delta)
		} else {
			conflict.Weight().RemoveCumulativeWeight(delta)
		}

		conflict.Weight().AddCumulativeWeight(delta)
	}
}

func assertSortedConflictsOrder[ConflictID, ResourceID IDType](t *testing.T, sortedConflicts *SortedConflicts[ConflictID, ResourceID], aliases ...string) {
	require.NoError(t, sortedConflicts.ForEach(func(c *Conflict[ConflictID, ResourceID]) error {
		currentAlias := aliases[0]
		aliases = aliases[1:]

		require.Equal(t, "OutputID("+currentAlias+")", c.ID().String())

		return nil
	}))

	require.Empty(t, aliases)
}

func newConflict(alias string, weight *Weight) *Conflict[utxo.OutputID, utxo.OutputID] {
	return NewConflict[utxo.OutputID, utxo.OutputID](
		outputID(alias),
		nil,
		advancedset.NewAdvancedSet[*ConflictSet[utxo.OutputID, utxo.OutputID]](),
		weight,
	)
}

func outputID(alias string) utxo.OutputID {
	outputID := utxo.OutputID{
		TransactionID: utxo.TransactionID{Identifier: blake2b.Sum256([]byte(alias))},
		Index:         0,
	}
	outputID.RegisterAlias(alias)

	return outputID
}
