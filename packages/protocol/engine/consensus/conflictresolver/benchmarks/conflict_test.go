package benchmarks

import (
	"fmt"
	"sort"
	"testing"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/generics/options"
)

var table = []struct {
	name     string
	scenario Scenario
	dsSent   int
}{
	{name: "1", dsSent: 1},
	{name: "500", dsSent: 500},
	{name: "1000", dsSent: 1000},
	{name: "1500", dsSent: 1500},
	{name: "2000", dsSent: 2000},
	{name: "2500", dsSent: 2500},
	{name: "3000", dsSent: 3000},
	{name: "4000", dsSent: 4000},
	{name: "4500", dsSent: 4500},

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

	for i, o := range ordered {
		m := (*s)[o.name]
		newName := fmt.Sprintf("%s_%d", o.name, i)
		createTestConflictB(conflictDAG, newName, m)
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

// creates a conflict and registers a ConflictIDAlias with the name specified in conflictMeta.
func createTestConflictB(conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], alias string, conflictMeta *ConflictMeta) bool {
	var newConflictCreated bool
	if conflictMeta.ConflictID == utxo.EmptyTransactionID {
		fmt.Println("WARNING:a conflict must have its ID defined in its ConflictMeta. Skipping")
		return false
	}
	newConflictCreated = conflictDAG.CreateConflict(conflictMeta.ConflictID, conflictMeta.ParentConflicts, conflictMeta.Conflicting)
	conflict, exist := conflictDAG.Conflict(conflictMeta.ConflictID)
	if !exist {
		panic("Conflict not found")
	}
	conflictMeta.ConflictID = conflict.ID()
	conflictMeta.ConflictID.RegisterAlias(alias)
	return newConflictCreated
}

func BenchmarkConflictResolution(b *testing.B) {
	for _, v := range table {
		b.Run(fmt.Sprintf("input_size_%s", v.name), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				scenarios, weightFunc := dynamicScenario(v.dsSent)
				performConflictResolutionScenario(b, scenarios, v.dsSent, weightFunc)
			}
		})
	}
}

func performConflictResolutionScenario(b *testing.B, s []Scenario, dsSent int, weightFunc func(utxo.TransactionID) int64) {
	storageInstance := storage.New(b.TempDir(), 1)
	defer storageInstance.Shutdown()
	var opts []options.Option[conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]]
	conflictDAG := conflictdag.New(opts...)
	o := conflictresolver.New(conflictDAG, weightFunc)
	b.StartTimer()
	for i := 0; i < dsSent; i++ {
		s[i].CreateConflictsB(conflictDAG)
		o.LikedConflictMember(s[i].ConflictIDB(conflictAlias("A", i)))
	}
}
