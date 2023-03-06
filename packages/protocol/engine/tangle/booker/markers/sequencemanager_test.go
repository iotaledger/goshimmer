package markers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/stringify"
)

func TestManager(t *testing.T) {
	testBlocks := []*block{
		newBlock("blk1"),
		newBlock("blk2"),
		newBlock("blk3", "blk1", "blk2"),
		newBlock("blk4", "blk3"),
		newBlock("blk5", "blk1", "blk2"),
		newBlock("blk6", "blk3", "blk5"),
		newBlock("blk7", "blk4"),
		newBlock("blk8", "blk4"),
		newBlock("blk9", "blk8"),
		newBlock("blk10", "blk4"),
		newBlock("blk11", "blk6"),
		newBlock("blk12", "blk11"),
		newBlock("blk13", "blk12"),
		newBlock("blk14", "blk11"),
		newBlock("blk15", "blk13", "blk14"),
		newBlock("blk16", "blk9", "blk15"),
		newBlock("blk17", "blk7", "blk16"),
		newBlock("blk18", "blk7", "blk13", "blk14"),
		newBlock("blk19", "blk7", "blk13", "blk14"),
		newBlock("blk20", "blk19"),
		newBlock("blk21", "blk20"),
	}

	blockDB := makeBlockDB(testBlocks...)
	manager := NewSequenceManager(WithMaxPastMarkerDistance(3))

	for _, m := range testBlocks {
		inheritPastMarkers(m, manager, blockDB)
	}

	type expectedStructureDetailsType struct {
		PastMarkers        *Markers
		PastMarkersGap     uint64
		ReferencedMarkers  *Markers
		ReferencingMarkers *Markers
	}

	expectedStructureDetails := map[string]expectedStructureDetailsType{
		"blk1": {
			PastMarkers: NewMarkers(
				NewMarker(0, 1),
			),
			PastMarkersGap:    0,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 2),
				NewMarker(2, 7),
			),
		},
		"blk2": {
			PastMarkers: NewMarkers(
				NewMarker(1, 1),
			),
			PastMarkersGap:    0,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(0, 2),
				NewMarker(2, 7),
			),
		},
		"blk3": {
			PastMarkers: NewMarkers(
				NewMarker(1, 2),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 1),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(0, 3),
				NewMarker(2, 7),
			),
		},
		"blk4": {
			PastMarkers: NewMarkers(
				NewMarker(1, 3),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 1),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(0, 8),
				NewMarker(2, 7),
			),
		},
		"blk5": {
			PastMarkers: NewMarkers(
				NewMarker(0, 2),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 1),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 7),
				NewMarker(1, 9),
			),
		},
		"blk6": {
			PastMarkers: NewMarkers(
				NewMarker(0, 3),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 7),
				NewMarker(1, 9),
			),
		},
		"blk7": {
			PastMarkers: NewMarkers(
				NewMarker(1, 4),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 1),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 7),
			),
		},
		"blk8": {
			PastMarkers: NewMarkers(
				NewMarker(1, 3),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
		},
		"blk9": {
			PastMarkers: NewMarkers(
				NewMarker(1, 3),
			),
			PastMarkersGap:     2,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
		},
		"blk10": {
			PastMarkers: NewMarkers(
				NewMarker(1, 3),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
		},
		"blk11": {
			PastMarkers: NewMarkers(
				NewMarker(0, 4),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 7),
				NewMarker(1, 9),
			),
		},
		"blk12": {
			PastMarkers: NewMarkers(
				NewMarker(0, 5),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 7),
				NewMarker(1, 9),
			),
		},
		"blk13": {
			PastMarkers: NewMarkers(
				NewMarker(0, 6),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 7),
				NewMarker(1, 9),
			),
		},
		"blk14": {
			PastMarkers: NewMarkers(
				NewMarker(0, 4),
			),
			PastMarkersGap:    1,
			ReferencedMarkers: NewMarkers(),
			ReferencingMarkers: NewMarkers(
				NewMarker(2, 7),
				NewMarker(1, 9),
			),
		},
		"blk15": {
			PastMarkers: NewMarkers(
				NewMarker(0, 7),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 2),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 9),
			),
		},
		"blk16": {
			PastMarkers: NewMarkers(
				NewMarker(0, 8),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 3),
			),
			ReferencingMarkers: NewMarkers(
				NewMarker(1, 9),
			),
		},
		"blk17": {
			PastMarkers: NewMarkers(
				NewMarker(1, 9),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(0, 8),
			),
			ReferencingMarkers: NewMarkers(),
		},
		"blk18": {
			PastMarkers: NewMarkers(
				NewMarker(1, 4),
				NewMarker(0, 6),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
		},
		"blk19": {
			PastMarkers: NewMarkers(
				NewMarker(1, 4),
				NewMarker(0, 6),
			),
			PastMarkersGap:     1,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
		},
		"blk20": {
			PastMarkers: NewMarkers(
				NewMarker(1, 4),
				NewMarker(0, 6),
			),
			PastMarkersGap:     2,
			ReferencedMarkers:  NewMarkers(),
			ReferencingMarkers: NewMarkers(),
		},
		"blk21": {
			PastMarkers: NewMarkers(
				NewMarker(2, 7),
			),
			PastMarkersGap: 0,
			ReferencedMarkers: NewMarkers(
				NewMarker(1, 4),
				NewMarker(0, 6),
			),
			ReferencingMarkers: NewMarkers(),
		},
	}

	for blockID, blockExpected := range expectedStructureDetails {
		assert.Equal(t, blockExpected.PastMarkers, blockDB[blockID].structureDetails.PastMarkers(), blockID+" has unexpected past Markers")
		assert.Equal(t, blockExpected.PastMarkersGap, blockDB[blockID].structureDetails.PastMarkerGap(), blockID+" has unexpected PastMarkerGap")

		if blockExpected.PastMarkersGap == 0 {
			pastMarker := blockExpected.PastMarkers.Marker()

			sequence, exists := manager.Sequence(pastMarker.SequenceID())
			assert.Truef(t, exists, "Sequence %s does not exist", pastMarker.SequenceID())
			assert.Equal(t, blockExpected.ReferencedMarkers, sequence.ReferencedMarkers(pastMarker.Index()), blockID+" has unexpected referenced Markers")
			assert.Equal(t, blockExpected.ReferencingMarkers, sequence.ReferencingMarkers(pastMarker.Index()), blockID+" has unexpected referencing Markers")
		}
	}
}

func TestManagerConvergence(t *testing.T) {
	tf := NewTestFramework(t)

	structureDetails1, _ := tf.SequenceManager().InheritStructureDetails(nil, false)
	assert.True(t, structureDetails1.PastMarkers().Equals(NewMarkers(NewMarker(0, 1))))

	structureDetails2, _ := tf.SequenceManager().InheritStructureDetails(nil, false)
	assert.True(t, structureDetails2.PastMarkers().Equals(NewMarkers(NewMarker(1, 1))))

	structureDetails3, _ := tf.SequenceManager().InheritStructureDetails(nil, false)
	assert.True(t, structureDetails3.PastMarkers().Equals(NewMarkers(NewMarker(2, 1))))

	structureDetails4, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2}, false)
	assert.True(t, structureDetails4.PastMarkers().Equals(NewMarkers(NewMarker(1, 2))))

	structureDetails5, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails3}, false)
	assert.True(t, structureDetails5.PastMarkers().Equals(NewMarkers(NewMarker(2, 2))))

	structureDetails6, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails1, structureDetails2, structureDetails3}, false)
	assert.True(t, structureDetails6.PastMarkers().Equals(NewMarkers(NewMarker(0, 2))))

	structureDetails7, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails2, structureDetails3}, false)
	assert.True(t, structureDetails7.PastMarkers().Equals(NewMarkers(NewMarker(1, 1), NewMarker(2, 1))))

	structureDetails8, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails4, structureDetails5}, false)
	assert.True(t, structureDetails8.PastMarkers().Equals(NewMarkers(NewMarker(2, 3))))

	structureDetails9, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails5, structureDetails6}, false)
	assert.True(t, structureDetails9.PastMarkers().Equals(NewMarkers(NewMarker(0, 3))))

	structureDetails10, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails6, structureDetails7}, false)
	assert.True(t, structureDetails10.PastMarkers().Equals(NewMarkers(NewMarker(0, 2), NewMarker(1, 1), NewMarker(2, 1))))

	structureDetails11, _ := tf.SequenceManager().InheritStructureDetails([]*StructureDetails{structureDetails9, structureDetails10}, false)
	assert.True(t, structureDetails11.PastMarkers().Equals(NewMarkers(NewMarker(0, 4))))
}

func inheritPastMarkers(block *block, manager *SequenceManager, blockDB map[string]*block) {
	// merge past Markers of referenced parents
	pastMarkers := make([]*StructureDetails, len(block.parents))
	for i, parentID := range block.parents {
		pastMarkers[i] = blockDB[parentID].structureDetails
	}

	block.structureDetails, _ = manager.InheritStructureDetails(pastMarkers, false)
}

func makeBlockDB(blocks ...*block) (blockDB map[string]*block) {
	blockDB = make(map[string]*block)
	for _, blk := range blocks {
		blockDB[blk.id] = blk
	}

	return
}

type block struct {
	id               string
	forceNewMarker   bool
	parents          []string
	structureDetails *StructureDetails
}

func newBlock(id string, parents ...string) *block {
	return &block{
		id:               id,
		parents:          parents,
		structureDetails: NewStructureDetails(),
	}
}

func (m *block) String() string {
	return stringify.Struct("block",
		stringify.NewStructField("id", m.id),
		stringify.NewStructField("forceNewMarker", m.forceNewMarker),
		stringify.NewStructField("parents", m.parents),
	)
}
