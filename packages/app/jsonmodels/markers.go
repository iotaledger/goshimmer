package jsonmodels

import (
	markers2 "github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/booker/markers"
)

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents the JSON model of the markersold.StructureDetails.
type StructureDetails struct {
	Rank          uint64   `json:"rank"`
	PastMarkerGap uint64   `json:"pastMarkerGap"`
	IsPastMarker  bool     `json:"isPastMarker"`
	PastMarkers   *Markers `json:"pastMarkers"`
}

// NewStructureDetails returns the StructureDetails from the given markersold.StructureDetails.
func NewStructureDetails(structureDetails *markers2.StructureDetails) *StructureDetails {
	if structureDetails == nil {
		return nil
	}

	return &StructureDetails{
		Rank:          structureDetails.Rank(),
		IsPastMarker:  structureDetails.IsPastMarker(),
		PastMarkerGap: structureDetails.PastMarkerGap(),
		PastMarkers:   NewMarkers(structureDetails.PastMarkers()),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents the JSON model of the markersold.Markers.
type Markers struct {
	Markers      map[markers2.SequenceID]markers2.Index `json:"markers"`
	HighestIndex markers2.Index                         `json:"highestIndex"`
	LowestIndex  markers2.Index                         `json:"lowestIndex"`
}

// NewMarkers returns the Markers from the given markersold.Markers.
func NewMarkers(m *markers2.Markers) *Markers {
	return &Markers{
		Markers: func() (mappedMarkers map[markers2.SequenceID]markers2.Index) {
			mappedMarkers = make(map[markers2.SequenceID]markers2.Index)
			m.ForEach(func(sequenceID markers2.SequenceID, index markers2.Index) bool {
				mappedMarkers[sequenceID] = index

				return true
			})

			return
		}(),
		HighestIndex: m.HighestIndex(),
		LowestIndex:  m.LowestIndex(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
