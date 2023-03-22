package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
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
func NewStructureDetails(structureDetails *markers.StructureDetails) *StructureDetails {
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
	Markers      map[markers.SequenceID]markers.Index `json:"markers"`
	HighestIndex markers.Index                        `json:"highestIndex"`
	LowestIndex  markers.Index                        `json:"lowestIndex"`
}

// NewMarkers returns the Markers from the given markersold.Markers.
func NewMarkers(m *markers.Markers) *Markers {
	return &Markers{
		Markers: func() (mappedMarkers map[markers.SequenceID]markers.Index) {
			mappedMarkers = make(map[markers.SequenceID]markers.Index)
			m.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
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
