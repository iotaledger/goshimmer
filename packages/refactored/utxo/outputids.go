package utxo

import (
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
)

type OutputIDs struct {
	*orderedmap.OrderedMap[OutputID, types.Empty]
}

func NewOutputIDs(ids ...OutputID) (new OutputIDs) {
	new = OutputIDs{orderedmap.New[OutputID, types.Empty]()}
	for _, outputID := range ids {
		new.Set(outputID, types.Void)
	}

	return new
}

func (o OutputIDs) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if o.OrderedMap == nil {
		o.OrderedMap = orderedmap.New[OutputID, types.Empty]()
	}

	elementCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return errors.Errorf("failed to parse amount of TransactionIDs: %w", err)
	}

	for i := 0; i < int(elementCount); i++ {
		var outputID OutputID
		if err = outputID.FromMarshalUtil(marshalUtil); err != nil {
			return errors.Errorf("failed to parse TransactionID: %w", err)
		}
		o.Add(outputID)
	}

	return nil
}

func (o OutputIDs) IsEmpty() (empty bool) {
	return o.OrderedMap == nil || o.OrderedMap.Size() == 0
}

func (o OutputIDs) Add(outputID OutputID) (added bool) {
	return o.Set(outputID, types.Void)
}

func (o OutputIDs) AddAll(outputIDs OutputIDs) (added bool) {
	_ = outputIDs.ForEach(func(outputID OutputID) (err error) {
		added = added || o.Set(outputID, types.Void)
		return nil
	})

	return added
}

func (o OutputIDs) DeleteAll(other OutputIDs) (removedElements OutputIDs) {
	removedElements = NewOutputIDs()
	_ = other.ForEach(func(outputID OutputID) (err error) {
		if o.Delete(outputID) {
			removedElements.Add(outputID)
		}
		return nil
	})

	return removedElements
}

func (o OutputIDs) ForEach(callback func(outputID OutputID) (err error)) (err error) {
	if o.OrderedMap == nil {
		return nil
	}

	o.OrderedMap.ForEach(func(outputID OutputID, _ types.Empty) bool {
		if err = callback(outputID); err != nil {
			return false
		}

		return true
	})

	return err
}

func (o OutputIDs) Intersect(other OutputIDs) (intersection OutputIDs) {
	return o.Filter(other.Has)
}

func (o OutputIDs) Filter(predicate func(outputID OutputID) bool) (filtered OutputIDs) {
	filtered = NewOutputIDs()
	_ = o.ForEach(func(outputID OutputID) (err error) {
		if predicate(outputID) {
			filtered.Add(outputID)
		}

		return nil
	})

	return filtered
}

func (o OutputIDs) Clone() (cloned OutputIDs) {
	cloned = NewOutputIDs()
	cloned.AddAll(o)

	return cloned
}

func (o OutputIDs) Slice() (slice []OutputID) {
	slice = make([]OutputID, 0)
	_ = o.ForEach(func(outputID OutputID) error {
		slice = append(slice, outputID)
		return nil
	})

	return slice
}

func (o OutputIDs) Bytes() (serialized []byte) {
	marshalUtil := marshalutil.New()
	if o.OrderedMap == nil {
		return marshalUtil.WriteUint64(0).Bytes()
	}

	marshalUtil.WriteUint64(uint64(o.Size()))
	_ = o.ForEach(func(outputID OutputID) (err error) {
		marshalUtil.Write(outputID)
		return nil
	})

	return marshalUtil.Bytes()
}

func (o OutputIDs) String() (humanReadable string) {
	elementStrings := generics.Map(o.Slice(), OutputID.String)
	if len(elementStrings) == 0 {
		return "OutputIDs()"
	}

	return "OutputIDs(" + strings.Join(elementStrings, ", ") + ")"
}

func (o OutputIDs) Iterator() *Iterator[OutputID] {
	outputIDs := make([]OutputID, 0)
	_ = o.ForEach(func(outputID OutputID) (err error) {
		outputIDs = append(outputIDs, outputID)
		return nil
	})

	return NewIterator(outputIDs...)
}
