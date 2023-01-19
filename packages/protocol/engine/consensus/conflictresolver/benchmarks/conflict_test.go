package benchmarks

import (
	"fmt"
	"sort"
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/stretchr/testify/require"
)

var table = []struct {
	name     string
	scenario Scenario
}{
	{name: "s1", scenario: s1},
	// todo inc number of conflicts
	// cpu profile plots generated from transactions
}

func (s *Scenario) CreateConflictsB(conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]) {
	type order struct {
		order int
		name  string
	}

	var ordered []order
	for name, m := range *s {
		ordered = append(ordered, order{order: m.Order, name: name})
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].order < ordered[j].order
	})

	for _, o := range ordered {
		m := (*s)[o.name]
		createTestConflictB(conflictDAG, o.name, m)
	}
}

// IDsToNames returns a mapping of ConflictIDs to their alias.
func (s *Scenario) IDsToNames() map[utxo.TransactionID]string {
	mapping := map[utxo.TransactionID]string{}
	for name, m := range *s {
		mapping[m.ConflictID] = name
	}
	return mapping
}

func (s *Scenario) ConflictID(alias string) utxo.TransactionID {
	return (*s)[alias].ConflictID
}

// creates a conflict and registers a ConflictIDAlias with the name specified in conflictMeta.
func createTestConflictB(conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], alias string, conflictMeta *ConflictMeta) bool {
	var newConflictCreated bool

	if conflictMeta.ConflictID == utxo.EmptyTransactionID {
		panic("a conflict must have its ID defined in its ConflictMeta")
	}
	newConflictCreated = conflictDAG.CreateConflict(conflictMeta.ConflictID, conflictMeta.ParentConflicts, conflictMeta.Conflicting)
	conflictDAG.Storage.CachedConflict(conflictMeta.ConflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		conflictMeta.ConflictID = conflict.ID()
	})
	conflictMeta.ConflictID.RegisterAlias(alias)
	return newConflictCreated
}

func WeightFuncFromScenario(b *testing.B, scenario Scenario) conflictresolver.WeightFunc {
	conflictIDsToName := scenario.IDsToNames()
	return func(conflictID utxo.TransactionID) (weight int64) {
		name, nameOk := conflictIDsToName[conflictID]
		require.True(b, nameOk)
		meta, metaOk := scenario[name]
		require.True(b, metaOk)
		return meta.ApprovalWeight
	}
}

func BenchmarkConflictResolution(b *testing.B) {
	for _, v := range table {
		b.Run(fmt.Sprintf("input_size_%s", v.name), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				fmt.Println(i)
				b.StopTimer()
				performConflictResolutionScenario(b, v.scenario)
			}
		})
	}
}

func performConflictResolutionScenario(b *testing.B, s Scenario) {
	ls := ledger.New(storage.New(b.TempDir(), 1), ledger.WithCacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ls.Shutdown()
	b.StartTimer()
	s.CreateConflictsB(ls.ConflictDAG)
	o := conflictresolver.New(ls.ConflictDAG, WeightFuncFromScenario(b, s))
	o.LikedConflictMember(s.ConflictID("A"))
	// fmt.Println(liked, conflictMembers)
}
