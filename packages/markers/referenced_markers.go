package markers

import (
	"sort"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region ReferencedMarkers /////////////////////////////////////////////////////////////////////////////////////////////

// ReferencedMarkers is a data structure that allows to denote which Marker of a Sequence references which other Markers
// of its parent Sequences in the Sequence DAG.
type ReferencedMarkers struct {
	referencedIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap
	mutex                       sync.RWMutex
}

// NewReferencedMarkers is the constructor for the ReferencedMarkers.
func NewReferencedMarkers(markers *Markers) (referencedMarkers *ReferencedMarkers) {
	referencedMarkers = &ReferencedMarkers{
		referencedIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	initialSequenceIndex := markers.HighestIndex() + 1
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		thresholdMap := thresholdmap.New(thresholdmap.LowerThresholdMode)
		thresholdMap.Set(uint64(initialSequenceIndex), uint64(index))

		referencedMarkers.referencedIndexesBySequence[sequenceID] = thresholdMap

		return true
	})

	return
}

// ReferencedMarkersFromBytes unmarshals ReferencedMarkers from a sequence of bytes.
func ReferencedMarkersFromBytes(parentReferencesBytes []byte) (referencedMarkers *ReferencedMarkers, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(parentReferencesBytes)
	if referencedMarkers, err = ReferencedMarkersFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ReferencedMarkers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferencedMarkersFromMarshalUtil unmarshals ReferencedMarkers using a MarshalUtil (for easier unmarshaling).
func ReferencedMarkersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (referencedMarkers *ReferencedMarkers, err error) {
	referencedMarkers = &ReferencedMarkers{
		referencedIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	sequenceCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse Sequence count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	for i := uint64(0); i < sequenceCount; i++ {
		sequenceID, sequenceIDErr := SequenceIDFromMarshalUtil(marshalUtil)
		if sequenceIDErr != nil {
			err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", sequenceIDErr)
			return
		}

		referenceCount, referenceCountErr := marshalUtil.ReadUint64()
		if referenceCountErr != nil {
			err = xerrors.Errorf("failed to parse reference count (%v): %w", referenceCountErr, cerrors.ErrParseBytesFailed)
			return
		}
		thresholdMap := thresholdmap.New(thresholdmap.LowerThresholdMode)
		for j := uint64(0); j < referenceCount; j++ {
			referencingIndex, referencingIndexErr := marshalUtil.ReadUint64()
			if referencingIndexErr != nil {
				err = xerrors.Errorf("failed to read referencing Index (%v): %w", referencingIndexErr, cerrors.ErrParseBytesFailed)
				return
			}

			referencedIndex, referencedIndexErr := marshalUtil.ReadUint64()
			if referencedIndexErr != nil {
				err = xerrors.Errorf("failed to read referenced Index (%v): %w", referencedIndexErr, cerrors.ErrParseBytesFailed)
				return
			}

			thresholdMap.Set(referencingIndex, referencedIndex)
		}
		referencedMarkers.referencedIndexesBySequence[sequenceID] = thresholdMap
	}

	return
}

// Add adds new referenced Markers to the ReferencedMarkers.
func (r *ReferencedMarkers) Add(index Index, referencedMarkers *Markers) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	referencedMarkers.ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
		thresholdMap, exists := r.referencedIndexesBySequence[referencedSequenceID]
		if !exists {
			thresholdMap = thresholdmap.New(thresholdmap.LowerThresholdMode)
			r.referencedIndexesBySequence[referencedSequenceID] = thresholdMap
		}

		thresholdMap.Set(uint64(index), uint64(referencedIndex))

		return true
	})
}

// Get returns that Markers that were referenced by the given Index.
func (r *ReferencedMarkers) Get(index Index) (referencedMarkers *Markers) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	referencedMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
		if referencedIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencedMarkers.Set(sequenceID, Index(referencedIndex.(uint64)))
		}
	}

	return
}

// Bytes returns a marshaled version of the ReferencedMarkers.
func (r *ReferencedMarkers) Bytes() (marshaledReferencedMarkers []byte) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(len(r.referencedIndexesBySequence)))
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
		marshalUtil.Write(sequenceID)
		marshalUtil.WriteUint64(uint64(thresholdMap.Size()))
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			marshalUtil.WriteUint64(node.Key().(uint64))
			marshalUtil.WriteUint64(node.Value().(uint64))

			return true
		})
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the ReferencedMarkers.
func (r *ReferencedMarkers) String() (humanReadableReferencedMarkers string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	referencingIndexes := make([]Index, 0)
	referencedMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			referencingIndex := Index(node.Key().(uint64))
			referencedIndex := Index(node.Value().(uint64))
			if _, exists := referencedMarkersByReferencingIndex[referencingIndex]; !exists {
				referencedMarkersByReferencingIndex[referencingIndex] = NewMarkers()

				referencingIndexes = append(referencingIndexes, referencingIndex)
			}

			referencedMarkersByReferencingIndex[referencingIndex].Set(sequenceID, referencedIndex)

			return true
		})
	}
	sort.Slice(referencingIndexes, func(i, j int) bool {
		return referencingIndexes[i] < referencingIndexes[j]
	})

	for i, referencedIndex := range referencingIndexes {
		for j := 0; j < i; j++ {
			referencedMarkersByReferencingIndex[referencingIndexes[j]].ForEach(func(sequenceID SequenceID, index Index) bool {
				if _, exists := referencedMarkersByReferencingIndex[referencedIndex].Get(sequenceID); exists {
					return true
				}

				referencedMarkersByReferencingIndex[referencedIndex].Set(sequenceID, index)

				return true
			})
		}
	}

	referencedMarkers := stringify.StructBuilder("ReferencedMarkers")
	for i, referencingIndex := range referencingIndexes {
		thresholdStart := strconv.FormatUint(uint64(referencingIndex), 10)
		thresholdEnd := "INF"
		if len(referencingIndexes) > i+1 {
			thresholdEnd = strconv.FormatUint(uint64(referencingIndexes[i+1])-1, 10)
		}

		if thresholdStart == thresholdEnd {
			referencedMarkers.AddField(stringify.StructField("Index("+thresholdStart+")", referencedMarkersByReferencingIndex[referencingIndex]))
		} else {
			referencedMarkers.AddField(stringify.StructField("Index("+thresholdStart+" ... "+thresholdEnd+")", referencedMarkersByReferencingIndex[referencingIndex]))
		}
	}

	return referencedMarkers.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
