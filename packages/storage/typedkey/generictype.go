package typedkey

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/pkg/errors"
)

type GenericType[T any] struct {
	store kvstore.KVStore
	key   []byte
	value T
	mutex sync.RWMutex
}

func NewGenericType[T any](store kvstore.KVStore, keyBytes ...byte) (newGenericType *GenericType[T]) {
	if len(keyBytes) == 0 {
		panic("keyBytes must not be empty")
	}

	newGenericType = new(GenericType[T])
	newGenericType.store = store
	newGenericType.key = keyBytes
	newGenericType.value = newGenericType.load()

	return
}

func (t *GenericType[T]) Get() (value T) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.value
}

func (t *GenericType[T]) Set(value T) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.value = value

	valueBytes := new(bytes.Buffer)
	if err := binary.Write(valueBytes, binary.LittleEndian, value); err != nil {
		panic(err)
	}

	if err := t.store.Set(t.key, valueBytes.Bytes()); err != nil {
		panic(err)
	}
}

func (t *GenericType[T]) load() (loadedValue T) {
	loadedValueBytes, err := t.store.Get(t.key)
	if err != nil {
		if !errors.Is(err, kvstore.ErrKeyNotFound) {
			panic(err)
		}

		return
	}

	if err = binary.Read(bytes.NewReader(loadedValueBytes), binary.LittleEndian, &loadedValue); err != nil {
		panic(err)
	}

	return
}
