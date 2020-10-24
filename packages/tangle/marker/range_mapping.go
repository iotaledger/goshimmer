package marker

import (
	"fmt"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/datastructure"

	"github.com/iotaledger/hive.go/stringify"
)

type UInt64Key uint64

func (u UInt64Key) Compare(other datastructure.Key) int {
	otherUInt64Key, typeCastOK := other.(UInt64Key)
	if !typeCastOK {
		panic("can only compare to other UInt64Keys")
	}

	switch {
	case u == otherUInt64Key:
		return 0
	case u > otherUInt64Key:
		return 1
	default:
		return -1
	}
}

type RangeType uint8

const (
	OpenRangeType RangeType = iota
)

// Range represents an interface for different kinds of numerical ranges.
type Range interface {
	Start() Key

	End() Key

	// Compare returns 1 if the value is bigger, -1 if the value is smaller and 0 if the value is inside the range.
	Compare(value uint64) int

	// Bytes returns a marshaled version of this Range.
	Bytes() []byte

	// String returns a human readable version of the Range.
	String() string
}

type RangeMap struct {
	head *RangeSeparator
	tail *RangeSeparator
}

func (b *RangeMap) Get(key uint64) (mappedValue interface{}, err error) {
	return b.tail.MappedValue(key)
}

func (b *RangeMap) IterateFromTail(consumer func(currentRange Range, mappedValue interface{}) bool) {
	currentNode := b.tail
	for currentNode != nil && consumer(currentNode.mappedValue, currentNode.mappedValue) {
		currentNode = currentNode.smallerValues
	}
}

func (b *RangeMap) Put(key Range, value interface{}) {
	b.IterateFromTail(func(currentRange Range, mappedValue interface{}) bool {

	})
}

func (b *RangeMap) AddMapping(rangeStart uint64, mappedValue interface{}) {
	var previousParent *RangeSeparator
	currentNode := b.tail
	for {
		if currentNode == nil {
			break
		}

		if currentNode.separatorValue == rangeStart {
			currentNode.mappedValue = mappedValue
			return
		}

		if currentNode.separatorValue < rangeStart {
			break
		}

		previousParent = currentNode
		currentNode = currentNode.smallerValues
	}

	newNode := &RangeSeparator{
		separatorValue: rangeStart,
		smallerValues:  currentNode,
		mappedValue:    mappedValue,
	}

	if previousParent == nil {
		b.tail = newNode
		return
	}

	previousParent.smallerValues = newNode

	return
}

func (b *RangeMap) String() string {
	structBuilder := stringify.StructBuilder("RangeMap")
	if b.tail == nil {
		return structBuilder.String()
	}

	var previousParent *RangeSeparator
	currentNode := b.tail
	numberLength := len(fmt.Sprintf("%d", currentNode.separatorValue))
	rangeStartLength := numberLength
	rangeEndLength := numberLength
	if rangeEndLength < 4 {
		rangeEndLength = 4
	}

	for currentNode != nil {
		if previousParent == nil {
			structBuilder.AddField(
				stringify.StructField(
					fmt.Sprintf("%-"+strconv.Itoa(rangeStartLength)+"d", currentNode.separatorValue)+" ... "+fmt.Sprintf("%"+strconv.Itoa(rangeEndLength)+"s", "+INF"),
					currentNode.mappedValue,
				),
			)
		} else {
			structBuilder.AddField(stringify.StructField(fmt.Sprintf("%-"+strconv.Itoa(rangeStartLength)+"d", currentNode.separatorValue)+" ... "+fmt.Sprintf("%"+strconv.Itoa(rangeEndLength)+"d", previousParent.separatorValue-1), currentNode.mappedValue))
		}

		previousParent = currentNode
		currentNode = currentNode.smallerValues
	}

	return structBuilder.String()
}
