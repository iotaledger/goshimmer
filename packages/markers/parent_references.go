package markers

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region ParentReferences /////////////////////////////////////////////////////////////////////////////////////////////

// ParentReferences models the relationship between Sequences by providing a way to encode which Marker references which
// other Markers of other Sequences.
type ParentReferences struct {
	referencedIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap
	mutex                       sync.RWMutex
}

// NewParentReferences creates a new set of ParentReferences.
func NewParentReferences(referencedMarkers *Markers) (newParentReferences *ParentReferences) {
	newParentReferences = &ParentReferences{
		referencedIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	initialSequenceIndex := referencedMarkers.HighestIndex() + 1
	referencedMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		thresholdMap := thresholdmap.New(thresholdmap.LowerThresholdMode)

		thresholdMap.Set(uint64(initialSequenceIndex), uint64(index))
		newParentReferences.referencedIndexesBySequence[sequenceID] = thresholdMap

		return true
	})

	return
}

// ParentReferencesFromBytes unmarshals a ParentReferences from a sequence of bytes.
func ParentReferencesFromBytes(parentReferencesBytes []byte) (parentReferences *ParentReferences, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(parentReferencesBytes)
	if parentReferences, err = ParentReferencesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Markers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParentReferencesFromMarshalUtil unmarshals a ParentReferences object using a MarshalUtil (for easier unmarshaling).
func ParentReferencesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (parentReferences *ParentReferences, err error) {
	parentReferences = &ParentReferences{
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
		parentReferences.referencedIndexesBySequence[sequenceID] = thresholdMap
	}

	return
}

// AddReferences adds referenced Markers to the ParentReferences.
func (p *ParentReferences) AddReferences(referencedMarkers *Markers, referencingIndex Index) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	referencedMarkers.ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
		thresholdMap, exists := p.referencedIndexesBySequence[referencedSequenceID]
		if !exists {
			thresholdMap = thresholdmap.New(thresholdmap.LowerThresholdMode)
			p.referencedIndexesBySequence[referencedSequenceID] = thresholdMap
		}

		thresholdMap.Set(uint64(referencingIndex), uint64(referencedIndex))

		return true
	})
}

// HighestReferencedMarker returns the referenced Marker with the highest Index of a given Sequence.
func (p *ParentReferences) HighestReferencedMarker(sequenceID SequenceID, referencingIndex Index) (highestReferencedMarker *Marker) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	thresholdMap, exists := p.referencedIndexesBySequence[sequenceID]
	if !exists {
		panic(fmt.Sprintf("Sequence with %s does not exist in ParentReferences", sequenceID))
	}

	highestReferencedIndex, exists := thresholdMap.Get(uint64(referencingIndex))
	if !exists {
		panic(fmt.Sprintf("%s references an unknown Index", referencingIndex))
	}

	return &Marker{
		sequenceID: sequenceID,
		index:      Index(highestReferencedIndex.(uint64)),
	}
}

// HighestReferencedMarkers returns a collection of Markers that were referenced by the given Index.
func (p *ParentReferences) HighestReferencedMarkers(index Index) (highestReferencedMarkers *Markers) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	highestReferencedMarkers = NewMarkers()
	for sequenceID, thresholdMap := range p.referencedIndexesBySequence {
		referencedIndex, exists := thresholdMap.Get(uint64(index))
		if !exists {
			panic(fmt.Sprintf("%s is smaller than the lowest known Index", index))
		}
		highestReferencedMarkers.Set(sequenceID, Index(referencedIndex.(uint64)))
	}

	return
}

// SequenceIDs returns the SequenceIDs of all referenced Sequences (and not just the parents in the Sequence DAG).
func (p *ParentReferences) SequenceIDs() (sequenceIDs SequenceIDs) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	sequenceIDsSlice := make([]SequenceID, 0, len(p.referencedIndexesBySequence))
	for sequenceID := range p.referencedIndexesBySequence {
		sequenceIDsSlice = append(sequenceIDsSlice, sequenceID)
	}

	return NewSequenceIDs(sequenceIDsSlice...)
}

// Bytes returns a marshaled version of the ParentReferences.
func (p *ParentReferences) Bytes() (marshaledParentReferences []byte) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(len(p.referencedIndexesBySequence)))
	for sequenceID, thresholdMap := range p.referencedIndexesBySequence {
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

// String returns a human readable version of the ParentReferences.
func (p *ParentReferences) String() (humanReadableParentReferences string) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	referencingIndexes := make([]Index, 0)
	referencedMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range p.referencedIndexesBySequence {
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

	return stringify.Struct("ParentReferences",
		stringify.StructField("referencedSequences", p.SequenceIDs()),
		stringify.StructField("referencedMarkers", referencedMarkers),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
