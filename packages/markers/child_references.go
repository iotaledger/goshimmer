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

// region ChildReferences //////////////////////////////////////////////////////////////////////////////////////////////

// ChildReferences models the relationship between Sequences by providing a way to encode which Marker references which
// other Markers of other Sequences.
type ChildReferences struct {
	referencingIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap
	mutex                        sync.RWMutex
}

// NewChildReferences creates a new set of ChildReferences.
func NewChildReferences() (newChildReferences *ChildReferences) {
	newChildReferences = &ChildReferences{
		referencingIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	return
}

// ChildReferencesFromBytes unmarshals a ChildReferences from a sequence of bytes.
func ChildReferencesFromBytes(childReferencesBytes []byte) (childReferences *ChildReferences, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(childReferencesBytes)
	if childReferences, err = ChildReferencesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Markers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ChildReferencesFromMarshalUtil unmarshals a ChildReferences object using a MarshalUtil (for easier unmarshaling).
func ChildReferencesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (childReferences *ChildReferences, err error) {
	childReferences = &ChildReferences{
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

// AddReferencingMarker adds referenced Markers to the ChildReferences.
func (c *ChildReferences) AddReferencingMarker(referencedIndex Index, referencingMarker *Marker) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	thresholdMap, thresholdMapExists := c.referencingIndexesBySequence[referencingMarker.SequenceID()]
	if !thresholdMapExists {
		thresholdMap = thresholdmap.New(thresholdmap.UpperThresholdMode)
		c.referencingIndexesBySequence[referencingMarker.SequenceID()] = thresholdMap
	}

	thresholdMap.Set(uint64(referencedIndex), referencingMarker.Index())
}

// ReferencingMarkers returns the Markers of child Sequences that reference the given Index.
func (c *ChildReferences) ReferencingMarkers(index Index) (referencingMarkers *Markers) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	referencingMarkers = NewMarkers()
	for sequenceID, thresholdMap := range c.referencingIndexesBySequence {
		referencingIndex, referencingMarkersExists := thresholdMap.Get(uint64(index))
		if !referencingMarkersExists {
			continue
		}

		referencingMarkers.Set(sequenceID, referencingIndex.(Index))
	}

	return
}

// ReferencingSequences returns the SequenceIDs of all referencing Sequences.
func (c *ChildReferences) ReferencingSequences() (sequenceIDs SequenceIDs) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	sequenceIDsSlice := make([]SequenceID, 0, len(c.referencingIndexesBySequence))
	for sequenceID := range c.referencingIndexesBySequence {
		sequenceIDsSlice = append(sequenceIDsSlice, sequenceID)
	}

	return NewSequenceIDs(sequenceIDsSlice...)
}

// Bytes returns a marshaled version of the ChildReferences.
func (c *ChildReferences) Bytes() (marshaledChildReferences []byte) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(len(c.referencingIndexesBySequence)))
	for sequenceID, thresholdMap := range c.referencingIndexesBySequence {
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

// String returns a human readable version of the ChildReferences.
func (c *ChildReferences) String() (humanReadableChildReferences string) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	referencedIndexes := make([]Index, 0)
	referencedMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range c.referencingIndexesBySequence {
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			referencedIndex := Index(node.Key().(uint64))
			referencingIndex := node.Value().(Index)
			if _, exists := referencedMarkersByReferencingIndex[referencedIndex]; !exists {
				referencedMarkersByReferencingIndex[referencedIndex] = NewMarkers()

				referencedIndexes = append(referencedIndexes, referencedIndex)
			}

			referencedMarkersByReferencingIndex[referencedIndex].Set(sequenceID, referencingIndex)

			return true
		})
	}
	sort.Slice(referencedIndexes, func(i, j int) bool {
		return referencedIndexes[i] < referencedIndexes[j]
	})

	for i, referencedIndex := range referencedIndexes {
		for j := i + 1; j < len(referencedIndexes); j++ {
			referencedMarkersByReferencingIndex[referencedIndexes[j]].ForEach(func(sequenceID SequenceID, index Index) bool {
				if _, exists := referencedMarkersByReferencingIndex[referencedIndex].Get(sequenceID); exists {
					return true
				}

				referencedMarkersByReferencingIndex[referencedIndex].Set(sequenceID, index)

				return true
			})
		}
	}

	thresholdStart := "0"
	referencedMarkers := stringify.StructBuilder("ReferencingMarkers")
	for _, referencingIndex := range referencedIndexes {
		thresholdEnd := strconv.FormatUint(uint64(referencingIndex), 10)

		if thresholdStart == thresholdEnd {
			referencedMarkers.AddField(stringify.StructField("Index("+thresholdStart+")", referencedMarkersByReferencingIndex[referencingIndex]))
		} else {
			referencedMarkers.AddField(stringify.StructField("Index("+thresholdStart+" ... "+thresholdEnd+")", referencedMarkersByReferencingIndex[referencingIndex]))
		}

		thresholdStart = strconv.FormatUint(uint64(referencingIndex)+1, 10)
	}

	return stringify.Struct("ChildReferences",
		stringify.StructField("referencingSequences", c.ReferencingSequences()),
		stringify.StructField("referencingMarkers", referencedMarkers),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
