package utxo

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/types"
)

type TransactionIDs struct {
	*orderedmap.OrderedMap[TransactionID, types.Empty]
}

func NewTransactionIDs(ids ...TransactionID) (new TransactionIDs) {
	new = TransactionIDs{orderedmap.New[TransactionID, types.Empty]()}
	for _, transactionID := range ids {
		new.Set(transactionID, types.Void)
	}

	return new
}

func (o TransactionIDs) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if o.OrderedMap == nil {
		o.OrderedMap = orderedmap.New[TransactionID, types.Empty]()
	}

	elementCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return errors.Errorf("failed to parse amount of TransactionIDs: %w", err)
	}

	for i := 0; i < int(elementCount); i++ {
		var txID TransactionID
		if err = txID.FromMarshalUtil(marshalUtil); err != nil {
			return errors.Errorf("failed to parse TransactionID: %w", err)
		}
		o.Add(txID)
	}

	return nil
}

func (o TransactionIDs) Add(transactionID TransactionID) (added bool) {
	return o.Set(transactionID, types.Void)
}

func (o TransactionIDs) AddAll(transactionIDs TransactionIDs) (added bool) {
	_ = transactionIDs.ForEach(func(transactionID TransactionID) (err error) {
		added = added || o.Set(transactionID, types.Void)
		return nil
	})

	return added
}

func (o TransactionIDs) DeleteAll(other TransactionIDs) (removedElements TransactionIDs) {
	removedElements = NewTransactionIDs()
	_ = other.ForEach(func(txID TransactionID) (err error) {
		if o.Delete(txID) {
			removedElements.Add(txID)
		}
		return nil
	})

	return removedElements
}

func (o TransactionIDs) ForEach(callback func(txID TransactionID) (err error)) (err error) {
	o.OrderedMap.ForEach(func(txID TransactionID, _ types.Empty) bool {
		if err = callback(txID); err != nil {
			return false
		}

		return true
	})

	return err
}

func (o TransactionIDs) Equal(other TransactionIDs) (equal bool) {
	if other.Size() != o.Size() {
		return false
	}

	return other.ForEach(func(txID TransactionID) (err error) {
		if !o.Has(txID) {
			return errors.New("abort")
		}

		return nil
	}) == nil
}

func (o TransactionIDs) Is(txID TransactionID) bool {
	return o.Size() == 1 && o.Has(txID)
}

func (o TransactionIDs) Clone() (cloned TransactionIDs) {
	cloned = NewTransactionIDs()
	cloned.AddAll(o)

	return cloned
}

func (o TransactionIDs) Slice() (slice []TransactionID) {
	slice = make([]TransactionID, 0)
	_ = o.ForEach(func(transactionID TransactionID) error {
		slice = append(slice, transactionID)
		return nil
	})

	return slice
}

func (o TransactionIDs) Bytes() (serialized []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint64(uint64(o.Size()))
	_ = o.ForEach(func(txID TransactionID) (err error) {
		marshalUtil.Write(txID)
		return nil
	})

	return marshalUtil.Bytes()
}

func (o TransactionIDs) String() (humanReadable string) {
	return ""
}
