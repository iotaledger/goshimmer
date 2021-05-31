package jsonmodels

import (
	markersPackage "github.com/iotaledger/goshimmer/packages/markers"
)

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents the JSON model of the markers.StructureDetails.
type StructureDetails struct {
	Rank          uint64   `json:"rank"`
	PastMarkerGap uint64   `json:"pastMarkerGap"`
	IsPastMarker  bool     `json:"isPastMarker"`
	PastMarkers   *Markers `json:"pastMarkers"`
	FutureMarkers *Markers `json:"futureMarkers"`
}

// NewStructureDetails returns the StructureDetails from the given markers.StructureDetails.
func NewStructureDetails(structureDetails *markersPackage.StructureDetails) *StructureDetails {
	if structureDetails == nil {
		return nil
	}

	return &StructureDetails{
		Rank:          structureDetails.Rank,
		IsPastMarker:  structureDetails.IsPastMarker,
		PastMarkerGap: structureDetails.PastMarkerGap,
		PastMarkers:   NewMarkers(structureDetails.PastMarkers),
		FutureMarkers: NewMarkers(structureDetails.FutureMarkers),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents the JSON model of the markers.Markers.
type Markers struct {
	Markers      map[markersPackage.SequenceID]markersPackage.Index `json:"markers"`
	HighestIndex markersPackage.Index                               `json:"highestIndex"`
	LowestIndex  markersPackage.Index                               `json:"lowestIndex"`
}

// NewMarkers returns the Markers from the given markers.Markers.
func NewMarkers(markers *markersPackage.Markers) *Markers {
	return &Markers{
		Markers: func() (mappedMarkers map[markersPackage.SequenceID]markersPackage.Index) {
			mappedMarkers = make(map[markersPackage.SequenceID]markersPackage.Index)
			markers.ForEach(func(sequenceID markersPackage.SequenceID, index markersPackage.Index) bool {
				mappedMarkers[sequenceID] = index

				return true
			})

			return
		}(),
		HighestIndex: markers.HighestIndex(),
		LowestIndex:  markers.LowestIndex(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
