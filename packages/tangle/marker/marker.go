package marker

type Index uint64

type Marker struct {
	sequenceID SequenceID
	index      Index
}

type Markers map[SequenceID]Index
