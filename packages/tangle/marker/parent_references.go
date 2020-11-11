package marker

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// ParentReferences models the relationship between Sequences by providing a way to encode the
type ParentReferences map[SequenceID]*thresholdmap.ThresholdMap

// NewParentReferences creates a new ParentReferences.
func NewParentReferences(referencedMarkers Markers) (parentReferences ParentReferences) {
	parentReferences = make(ParentReferences)

	initialSequenceIndex := referencedMarkers.HighestIndex() + 1
	for _, marker := range referencedMarkers {
		if _, exists := parentReferences[marker.sequenceID]; exists {
			panic("the Markers passed to NewParentReferences need to be unique (normalized)")
		}
		thresholdMap := thresholdmap.New(thresholdmap.LowerThresholdMode)

		thresholdMap.Set(uint64(initialSequenceIndex), uint64(marker.index))
		parentReferences[marker.sequenceID] = thresholdMap
	}

	return
}

// ParentReferencesFromBytes unmarshals a ParentReferences from a sequence of bytes.
func ParentReferencesFromBytes(parentReferencesBytes []byte) (parentReferences ParentReferences, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(parentReferencesBytes)
	if parentReferences, err = ParentReferencesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Markers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParentReferencesFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParentReferencesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (parentReferences ParentReferences, err error) {
	sequenceCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse Sequence count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	parentReferences = make(ParentReferences)
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
		parentReferences[sequenceID] = thresholdMap
	}

	return
}

// AddReferences add referenced markers to the ParentReferences.
func (p ParentReferences) AddReferences(referencedMarkers Markers, referencingIndex Index) {
	for _, referencedMarker := range referencedMarkers {
		thresholdMap, exists := p[referencedMarker.sequenceID]
		if !exists {
			panic(fmt.Sprintf("tried to update referenced Marker of unknown parent Sequence: %s", referencedMarker.sequenceID))
		}

		if thresholdMap.MaxElement().Value().(uint64) >= uint64(referencedMarker.index) {
			continue
		}

		thresholdMap.Set(uint64(referencingIndex), uint64(referencedMarker.index))
	}
}

// HighestReferencedMarker returns a marker with the highest index of a specific marker sequence.
func (p ParentReferences) HighestReferencedMarker(sequenceID SequenceID, referencingIndex Index) (highestReferencedMarker *Marker) {
	thresholdMap, exists := p[sequenceID]
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

// HighestReferencedMarkers returns a list of highest index markers in different marker sequence.
func (p ParentReferences) HighestReferencedMarkers(index Index) (highestReferencedMarkers UniqueMarkers) {
	highestReferencedMarkers = make(UniqueMarkers)
	for sequenceID, thresholdMap := range p {
		referencedIndex, exists := thresholdMap.Get(uint64(index))
		if !exists {
			panic(fmt.Sprintf("%s is smaller than the lowest known Index", index))
		}
		highestReferencedMarkers[sequenceID] = Index(referencedIndex.(uint64))
	}

	return
}

// SequenceIDs returns the IDs of the marker sequence of ParentReferences.
func (p ParentReferences) SequenceIDs() SequenceIDs {
	sequenceIDs := make([]SequenceID, 0, len(p))
	for sequenceID := range p {
		sequenceIDs = append(sequenceIDs, sequenceID)
	}

	return NewSequenceIDs(sequenceIDs...)
}

// Bytes returns the ParentReferences in serialized byte form.
func (p ParentReferences) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint64(uint64(len(p)))
	for sequenceID, thresholdMap := range p {
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

// String returns the base58 encode of the ParentReferences.
func (p ParentReferences) String() string {
	referencingIndexes := make([]Index, 0)
	referencedMarkersByReferencingIndex := make(map[Index][]*Marker)
	for sequenceID, thresholdMap := range p {
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			referencingIndex := Index(node.Key().(uint64))
			referencedIndex := Index(node.Value().(uint64))
			if _, exists := referencedMarkersByReferencingIndex[referencingIndex]; !exists {
				referencedMarkersByReferencingIndex[referencingIndex] = make([]*Marker, 0)

				referencingIndexes = append(referencingIndexes, referencingIndex)
			}

			referencedMarkersByReferencingIndex[referencingIndex] = append(referencedMarkersByReferencingIndex[referencingIndex], &Marker{sequenceID, referencedIndex})

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
		stringify.StructField("referencedSequenceIDs", p.SequenceIDs()),
		stringify.StructField("referencedMarkers", referencedMarkers),
	)
}
