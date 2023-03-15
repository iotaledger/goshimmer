package newconflictdag_test

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	. "github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

func TestSortedConflict(t *testing.T) {
	sortedConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()

	conflict1 := newConflict("conflict1", NewWeight(12, nil, Rejected))
	conflict2 := newConflict("conflict2", NewWeight(10, nil, Pending))
	conflict3 := newConflict("conflict3", NewWeight(1, nil, Accepted))
	conflict4 := newConflict("conflict4", NewWeight(11, nil, Rejected))
	conflict5 := newConflict("conflict5", NewWeight(11, nil, Pending))
	conflict6 := newConflict("conflict6", NewWeight(2, nil, Accepted))

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
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict2", "conflict5", "conflict1", "conflict4")

	conflict2.Weight().RemoveCumulativeWeight(3)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict6", "conflict3", "conflict5", "conflict2", "conflict1", "conflict4")

	conflict5.Weight().SetConfirmationState(Accepted)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict5", "conflict6", "conflict3", "conflict2", "conflict1", "conflict4")
}

func TestSortedDecreaseHeaviest(t *testing.T) {
	sortedConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()

	conflict1 := newConflict("conflict1", NewWeight(1, nil, Accepted))
	conflict2 := newConflict("conflict2", NewWeight(2, nil, Pending))

	sortedConflicts.Add(conflict1)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1")

	sortedConflicts.Add(conflict2)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict1", "conflict2")

	conflict1.Weight().SetConfirmationState(Pending)
	assertSortedConflictsOrder(t, sortedConflicts, "conflict2", "conflict1")
}

func TestSortedConflictParallel(t *testing.T) {
	const conflictCount = 1000
	const updateCount = 100000

	sortedConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()
	sortedParallelConflicts := NewSortedConflicts[utxo.OutputID, utxo.OutputID]()

	conflicts := make(map[string]*Conflict[utxo.OutputID, utxo.OutputID])
	parallelConflicts := make(map[string]*Conflict[utxo.OutputID, utxo.OutputID])

	for i := 0; i < conflictCount; i++ {
		alias := "conflict" + strconv.Itoa(i)

		conflicts[alias] = newConflict(alias, NewWeight(0, nil, Pending))
		parallelConflicts[alias] = newConflict(alias, NewWeight(0, nil, Pending))

		sortedConflicts.Add(conflicts[alias])
		sortedParallelConflicts.Add(conflicts[alias])
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
			permutation(conflicts[targetAlias])

			wg.Done()
		}(permutation)
	}

	wg.Wait()

	originalSortingAfter := sortedConflicts.String()
	parallelSortingAfter := sortedParallelConflicts.String()
	require.Equal(t, originalSortingAfter, parallelSortingAfter)
	require.NotEqualf(t, originalSortingBefore, originalSortingAfter, "original sorting should have changed")
}

func generateRandomWeightPermutation() func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
	switch rand.Intn(2) {
	case 0:
		return generateRandomCumulativeWeightPermutation(uint64(rand.Intn(100)))
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
			conflict.Weight().SetConfirmationState(Pending)
		case 1:
			conflict.Weight().SetConfirmationState(Accepted)
		case 2:
			conflict.Weight().SetConfirmationState(Rejected)
		}
	}
}

func generateRandomCumulativeWeightPermutation(delta uint64) func(conflict *Conflict[utxo.OutputID, utxo.OutputID]) {
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
