package newconflictdag_test

import (
	"testing"

	"github.com/stretchr/testify/require"

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
}

func assertSortedConflictsOrder[ConflictID, ResourceID IDType](t *testing.T, sortedConflicts *SortedConflicts[ConflictID, ResourceID], aliases ...string) {
	sortedConflicts.ForEach(func(c *Conflict[ConflictID, ResourceID]) error {
		currentAlias := aliases[0]
		aliases = aliases[1:]

		require.Equal(t, "OutputID("+currentAlias+")", c.ID().String())

		return nil
	})

	require.Empty(t, aliases)
}

func newConflict(alias string, weight *Weight) *Conflict[utxo.OutputID, utxo.OutputID] {
	var randomOutputID utxo.OutputID
	if err := randomOutputID.FromRandomness(); err != nil {
		panic(err)
	}
	randomOutputID.RegisterAlias(alias)

	return NewConflict[utxo.OutputID, utxo.OutputID](
		randomOutputID,
		nil,
		advancedset.NewAdvancedSet[*ConflictSet[utxo.OutputID, utxo.OutputID]](),
		weight,
	)
}
