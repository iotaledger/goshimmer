package markers

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/set"
)

type TestFramework struct {
	SequenceManager *SequenceManager

	t *testing.T

	structureDetailsByAlias map[string]*StructureDetails
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(t *testing.T) (newFramework *TestFramework) {
	return &TestFramework{
		SequenceManager:         NewSequenceManager(),
		structureDetailsByAlias: make(map[string]*StructureDetails),

		t: t,
	}
}

func (t *TestFramework) InheritStructureDetails(alias string, inheritedStructureDetails []*StructureDetails) (structureDetails *StructureDetails, created bool) {
	structureDetails, created = t.SequenceManager.InheritStructureDetails(inheritedStructureDetails)
	t.structureDetailsByAlias[alias] = structureDetails

	return
}

func (t *TestFramework) StructureDetails(alias string) (structureDetails *StructureDetails) {
	structureDetails, ok := t.structureDetailsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("StructureDetails alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) StructureDetailsSet(aliases ...string) (structureDetailsSlice []*StructureDetails) {
	structureDetailsSet := set.NewAdvancedSet[*StructureDetails]()
	for _, alias := range aliases {
		structureDetailsSet.Add(t.StructureDetails(alias))
	}

	return structureDetailsSet.Slice()
}
