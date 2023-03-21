package conflict_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	. "github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/lo"
)

func TestConflictSets(t *testing.T) {
	red := NewConflictSet[utxo.OutputID, utxo.OutputID](outputID("red"))
	blue := NewConflictSet[utxo.OutputID, utxo.OutputID](outputID("blue"))
	green := NewConflictSet[utxo.OutputID, utxo.OutputID](outputID("green"))
	yellow := NewConflictSet[utxo.OutputID, utxo.OutputID](outputID("yellow"))
	fmt.Println("adding A...")
	conflictA := New[utxo.OutputID, utxo.OutputID](
		outputID("A"),
		nil,
		map[utxo.OutputID]*Set[utxo.OutputID, utxo.OutputID]{
			red.ID(): red,
		},
		weight.New().AddCumulativeWeight(7).SetAcceptanceState(acceptance.Pending),
	)
	fmt.Println("adding B...")
	conflictB := New[utxo.OutputID, utxo.OutputID](
		outputID("B"),
		nil,
		map[utxo.OutputID]*Set[utxo.OutputID, utxo.OutputID]{
			red.ID():  red,
			blue.ID(): blue,
		},
		weight.New().AddCumulativeWeight(3).SetAcceptanceState(acceptance.Pending),
	)
	fmt.Println("adding C...")
	conflictC := New[utxo.OutputID, utxo.OutputID](
		outputID("C"),
		nil,
		map[utxo.OutputID]*Set[utxo.OutputID, utxo.OutputID]{
			green.ID(): green,
			blue.ID():  blue,
		},
		weight.New().AddCumulativeWeight(5).SetAcceptanceState(acceptance.Pending),
	)

	fmt.Println("adding D...")
	conflictD := New[utxo.OutputID, utxo.OutputID](
		outputID("D"),
		nil,
		map[utxo.OutputID]*Set[utxo.OutputID, utxo.OutputID]{
			green.ID():  green,
			yellow.ID(): yellow,
		},
		weight.New().AddCumulativeWeight(7).SetAcceptanceState(acceptance.Pending),
	)

	fmt.Println("adding E...")

	conflictE := New[utxo.OutputID, utxo.OutputID](
		outputID("E"),
		nil,
		map[utxo.OutputID]*Set[utxo.OutputID, utxo.OutputID]{
			yellow.ID(): yellow,
		},
		weight.New().AddCumulativeWeight(9).SetAcceptanceState(acceptance.Pending),
	)

	preferredInsteadMap := map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
		conflictA: conflictA,
		conflictB: conflictA,
		conflictC: conflictC,
		conflictD: conflictE,
		conflictE: conflictE,
	}

	assertPreferredInstead(t, preferredInsteadMap)

	fmt.Println("set weight D=10...")

	conflictD.Weight().SetCumulativeWeight(10)

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
		conflictC: conflictD,
		conflictD: conflictD,
		conflictE: conflictD,
	}))

	fmt.Println("set weight D=0...")

	conflictD.Weight().SetCumulativeWeight(0)

	assertPreferredInstead(t, lo.MergeMaps(preferredInsteadMap, map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]{
		conflictC: conflictC,
		conflictD: conflictE,
		conflictE: conflictE,
	}))
}

func assertPreferredInstead(t *testing.T, preferredInsteadMap map[*Conflict[utxo.OutputID, utxo.OutputID]]*Conflict[utxo.OutputID, utxo.OutputID]) {
	// TODO: wait in a similar fashion as the workerpools so we always wait recusively in case one conflict modifies the others that we did wait for before
	for conflict, _ := range preferredInsteadMap {
		conflict.WaitConsistent()
	}

	time.Sleep(5 * time.Second)
	fmt.Println("sleep done")
	for conflict, preferredInsteadConflict := range preferredInsteadMap {
		assert.Equalf(t, preferredInsteadConflict.ID(), conflict.PreferredInstead().ID(), "conflict %s should prefer %s instead of %s", conflict.ID(), preferredInsteadConflict.ID(), conflict.PreferredInstead().ID())
		fmt.Println(conflict.ID(), "->", conflict.PreferredInstead().ID())
	}
}
