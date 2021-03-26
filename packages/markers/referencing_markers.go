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

// region ReferencingMarkers ///////////////////////////////////////////////////////////////////////////////////////////

// ReferencingMarkers is a data structure that allows to denote which Markers of child Sequences in the Sequence DAG
// reference a given Marker in a Sequence.
type ReferencingMarkers struct {
	referencingIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap
	mutex                        sync.RWMutex
}

// NewReferencingMarkers is the constructor for the ReferencingMarkers.
func NewReferencingMarkers() (referencingMarkers *ReferencingMarkers) {
	referencingMarkers = &ReferencingMarkers{
		referencingIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	return
}

// ReferencingMarkersFromBytes unmarshals ReferencingMarkers from a sequence of bytes.
func ReferencingMarkersFromBytes(referencingMarkersBytes []byte) (referencingMarkers *ReferencingMarkers, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(referencingMarkersBytes)
	if referencingMarkers, err = ReferencingMarkersFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ReferencingMarkers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferencingMarkersFromMarshalUtil unmarshals ReferencingMarkers using a MarshalUtil (for easier unmarshaling).
func ReferencingMarkersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (childReferences *ReferencingMarkers, err error) {
	childReferences = &ReferencingMarkers{
		referencingIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
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
		thresholdMap := thresholdmap.New(thresholdmap.UpperThresholdMode)
		for j := uint64(0); j < referenceCount; j++ {
			referencedIndex, referencedIndexErr := marshalUtil.ReadUint64()
			if referencedIndexErr != nil {
				err = xerrors.Errorf("failed to read referenced Index (%v): %w", referencedIndexErr, cerrors.ErrParseBytesFailed)
				return
			}

			referencingIndex, referencingIndexErr := marshalUtil.ReadUint64()
			if referencingIndexErr != nil {
				err = xerrors.Errorf("failed to read referencing Index (%v): %w", referencingIndexErr, cerrors.ErrParseBytesFailed)
				return
			}

			thresholdMap.Set(referencedIndex, Index(referencingIndex))
		}
		childReferences.referencingIndexesBySequence[sequenceID] = thresholdMap
	}

	return
}

// Add adds a new referencing Marker to the ReferencingMarkers.
func (r *ReferencingMarkers) Add(index Index, referencingMarker *Marker) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	thresholdMap, thresholdMapExists := r.referencingIndexesBySequence[referencingMarker.SequenceID()]
	if !thresholdMapExists {
		thresholdMap = thresholdmap.New(thresholdmap.UpperThresholdMode)
		r.referencingIndexesBySequence[referencingMarker.SequenceID()] = thresholdMap
	}

	thresholdMap.Set(uint64(index), referencingMarker.Index())
}

// Get returns the Markers of child Sequences that reference the given Index.
func (r *ReferencingMarkers) Get(index Index) (referencingMarkers *Markers) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	referencingMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
		if referencingIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencingMarkers.Set(sequenceID, referencingIndex.(Index))
		}
	}

	return
}

// Bytes returns a marshaled version of the ReferencingMarkers.
func (r *ReferencingMarkers) Bytes() (marshaledReferencingMarkers []byte) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(len(r.referencingIndexesBySequence)))
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
		marshalUtil.Write(sequenceID)
		marshalUtil.WriteUint64(uint64(thresholdMap.Size()))
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			marshalUtil.WriteUint64(node.Key().(uint64))
			marshalUtil.WriteUint64(uint64(node.Value().(Index)))

			return true
		})
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the ReferencingMarkers.
func (r *ReferencingMarkers) String() (humanReadableReferencingMarkers string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	indexes := make([]Index, 0)
	referencingMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			index := Index(node.Key().(uint64))
			referencingIndex := node.Value().(Index)
			if _, exists := referencingMarkersByReferencingIndex[index]; !exists {
				referencingMarkersByReferencingIndex[index] = NewMarkers()

				indexes = append(indexes, index)
			}

			referencingMarkersByReferencingIndex[index].Set(sequenceID, referencingIndex)

			return true
		})
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i] < indexes[j]
	})

	for i, index := range indexes {
		for j := i + 1; j < len(indexes); j++ {
			referencingMarkersByReferencingIndex[indexes[j]].ForEach(func(referencingSequenceID SequenceID, referencingIndex Index) bool {
				if _, exists := referencingMarkersByReferencingIndex[index].Get(referencingSequenceID); exists {
					return true
				}

				referencingMarkersByReferencingIndex[index].Set(referencingSequenceID, referencingIndex)

				return true
			})
		}
	}

	thresholdStart := "0"
	referencingMarkers := stringify.StructBuilder("ReferencingMarkers")
	for _, index := range indexes {
		thresholdEnd := strconv.FormatUint(uint64(index), 10)

		if thresholdStart == thresholdEnd {
			referencingMarkers.AddField(stringify.StructField("Index("+thresholdStart+")", referencingMarkersByReferencingIndex[index]))
		} else {
			referencingMarkers.AddField(stringify.StructField("Index("+thresholdStart+" ... "+thresholdEnd+")", referencingMarkersByReferencingIndex[index]))
		}

		thresholdStart = strconv.FormatUint(uint64(index)+1, 10)
	}

	return referencingMarkers.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
