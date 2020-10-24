package marker

import (
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

type RangeSeparator struct {
	separatorValue uint64
	mappedValue    interface{}
	smallerValues  *RangeSeparator
	largerValue    *RangeSeparator
}

func (b *RangeSeparator) MappedValue(markerIndex uint64) (mappedValue interface{}, err error) {
	currentNode := b
	for currentNode != nil {
		if markerIndex >= currentNode.separatorValue {
			mappedValue = currentNode.mappedValue
			return
		}

		currentNode = currentNode.smallerValues
	}

	err = xerrors.Errorf("markerIndex is smaller than the start of the Sequence")
	return
}

func (b *RangeSeparator) String() string {
	return stringify.Struct("RangeSeparator",
		stringify.StructField("separatorValue", b.separatorValue),
		stringify.StructField("mappedValue", b.mappedValue),
		stringify.StructField("smallerValues", b.smallerValues),
	)
}
