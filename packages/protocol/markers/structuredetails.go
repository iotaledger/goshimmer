package markers

import (
	"fmt"
	"sync"
)

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents a container for the complete Marker related information of a node in a DAG that are used
// to interact with the public API of this package.
type StructureDetails struct {
	rank          uint64
	pastMarkerGap uint64
	isPastMarker  bool
	pastMarkers   *Markers

	sync.RWMutex
}

// NewStructureDetails creates an empty StructureDetails object.
func NewStructureDetails() (s *StructureDetails) {
	return &StructureDetails{
		pastMarkers: NewMarkers(),
	}
}

func (s *StructureDetails) Rank() (rank uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.rank
}

func (s *StructureDetails) SetRank(rank uint64) {
	s.Lock()
	defer s.Unlock()

	s.rank = rank
}

func (s *StructureDetails) PastMarkerGap() (pastMarkerGap uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.pastMarkerGap
}

func (s *StructureDetails) SetPastMarkerGap(pastMarkerGap uint64) {
	s.Lock()
	defer s.Unlock()

	s.pastMarkerGap = pastMarkerGap
}

func (s *StructureDetails) IsPastMarker() (isPastMarker bool) {
	s.RLock()
	defer s.RUnlock()

	return s.isPastMarker
}

func (s *StructureDetails) SetIsPastMarker(isPastMarker bool) {
	s.Lock()
	defer s.Unlock()

	s.isPastMarker = isPastMarker
}

func (s *StructureDetails) PastMarkers() (pastMarkers *Markers) {
	s.RLock()
	defer s.RUnlock()

	return s.pastMarkers
}

func (s *StructureDetails) SetPastMarkers(pastMarkers *Markers) {
	s.Lock()
	defer s.Unlock()

	s.pastMarkers = pastMarkers
}

// Clone creates a deep copy of the StructureDetails.
func (s *StructureDetails) Clone() (clone *StructureDetails) {
	return &StructureDetails{
		rank:          s.Rank(),
		pastMarkerGap: s.PastMarkerGap(),
		isPastMarker:  s.IsPastMarker(),
		pastMarkers:   s.PastMarkers().Clone(),
	}
}

func (s *StructureDetails) String() string {
	return fmt.Sprintf("StructureDetails{rank: %d, pastMarkerGap: %d, isPastMarker: %t, pastMarkers: %s}", s.Rank(), s.PastMarkerGap(), s.IsPastMarker(), s.PastMarkers())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
