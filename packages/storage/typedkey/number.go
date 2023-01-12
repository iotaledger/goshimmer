package typedkey

import (
	"bytes"
	"encoding/binary"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/kvstore"
)

type Number[T constraints.Numeric] struct {
	*GenericType[T]
}

func NewNumber[T constraints.Numeric](store kvstore.KVStore, keyBytes ...byte) (newNumber *Number[T]) {
	newNumber = new(Number[T])
	newNumber.GenericType = NewGenericType[T](store, keyBytes...)

	return
}

func (n *Number[T]) Inc() (newValue T) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	newValue = n.value + 1
	n.value = newValue

	valueBytes := new(bytes.Buffer)
	if err := binary.Write(valueBytes, binary.LittleEndian, newValue); err != nil {
		panic(err)
	}

	if err := n.store.Set(n.key, valueBytes.Bytes()); err != nil {
		panic(err)
	}

	return
}

func (n *Number[T]) Dec() (newValue T) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	newValue = n.value - 1
	n.value = newValue

	valueBytes := new(bytes.Buffer)
	if err := binary.Write(valueBytes, binary.LittleEndian, newValue); err != nil {
		panic(err)
	}

	if err := n.store.Set(n.key, valueBytes.Bytes()); err != nil {
		panic(err)
	}

	return
}

func (n *Number[T]) IsZero() bool {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	return n.value == 0
}
