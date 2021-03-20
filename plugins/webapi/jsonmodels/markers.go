package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents a container for the complete Marker related information of a node in a DAG that are used
// to interact with the public API of this package.
type StructureDetails struct {
	Rank          uint64  `json:"rank"`
	IsPastMarker  bool    `json:"isPastMarker"`
	PastMarkers   Markers `json:"pastMarkers"`
	FutureMarkers Markers `json:"futureMarkers"`
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents a collection of Markers that can contain exactly one Index per SequenceID.
type Markers struct {
	Markers      map[markers.SequenceID]markers.Index `json:"markers"`
	HighestIndex markers.Index                        `json:"highestIndex"`
	LowestIndex  markers.Index                        `json:"lowestIndex"`
}

func newMarkers(m *markers.Markers) (newMarkers Markers) {
	newMarkers.Markers = make(map[markers.SequenceID]markers.Index)
	m.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		newMarkers.Markers[sequenceID] = index
		return true
	})
	newMarkers.HighestIndex = m.HighestIndex()
	newMarkers.LowestIndex = m.LowestIndex()
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
