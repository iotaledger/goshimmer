package markers

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/options"
)

type TestFramework struct {
	sequenceManager *SequenceManager

	t *testing.T

	structureDetailsByAlias map[string]*StructureDetails
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(t *testing.T, opts ...options.Option[TestFramework]) (newFramework *TestFramework) {
	return options.Apply(&TestFramework{
		structureDetailsByAlias: make(map[string]*StructureDetails),

		t: t,
	}, opts)
}

func (t *TestFramework) InheritStructureDetails(alias string, inheritedStructureDetails []*StructureDetails) (structureDetails *StructureDetails, created bool) {
	structureDetails, created = t.SequenceManager().InheritStructureDetails(inheritedStructureDetails, false)
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
	structureDetailsSet := advancedset.New[*StructureDetails]()
	for _, alias := range aliases {
		structureDetailsSet.Add(t.StructureDetails(alias))
	}

	return structureDetailsSet.Slice()
}

func (t *TestFramework) SequenceManager() (sequenceTracker *SequenceManager) {
	if t.sequenceManager == nil {
		t.sequenceManager = NewSequenceManager()
	}

	return t.sequenceManager
}

func WithSequenceManager(sequenceManager *SequenceManager) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.sequenceManager != nil {
			panic("sequence manager already set")
		}
		tf.sequenceManager = sequenceManager
	}
}
