package newconflictdag_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

func TestSortedConflict(t *testing.T) {
	sortedConflicts := newconflictdag.NewSortedConflicts[utxo.OutputID, utxo.OutputID]()

	var outputID1 utxo.OutputID
	require.NoError(t, outputID1.FromRandomness())

	conflict1 := newConflict("conflict1", newconflictdag.NewWeight(12, nil, newconflictdag.Rejected))
	conflict2 := newConflict("conflict1", newconflictdag.NewWeight(10, nil, newconflictdag.Pending))
	conflict3 := newConflict("conflict1", newconflictdag.NewWeight(1, nil, newconflictdag.Accepted))

	sortedConflicts.Add(conflict1)
	sortedConflicts.Add(conflict2)
	sortedConflicts.Add(conflict3)

	fmt.Println(sortedConflicts.String())
}

func newConflict(alias string, weight *newconflictdag.Weight) *newconflictdag.Conflict[utxo.OutputID, utxo.OutputID] {
	return newconflictdag.NewConflict[utxo.OutputID, utxo.OutputID](
		randomOutputID(alias),
		nil,
		advancedset.NewAdvancedSet[*newconflictdag.ConflictSet[utxo.OutputID, utxo.OutputID]](),
		weight,
	)
}

func randomOutputID(alias string) (outputID utxo.OutputID) {
	if err := outputID.FromRandomness(); err != nil {
		panic(err)
	}

	outputID.RegisterAlias(alias)

	return
}
