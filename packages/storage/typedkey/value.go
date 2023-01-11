package typedkey

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"
)

type Marshalable[V any, VPtr constraints.MarshalablePtr[V]] struct {
	store kvstore.KVStore
	key   []byte
	value V
	mutex sync.RWMutex
}

func NewMarshalable[V any, VPtr constraints.MarshalablePtr[V]](store kvstore.KVStore, key []byte) (newMarshalable *Marshalable[V, VPtr]) {
	newMarshalable = new(Marshalable[V, VPtr])
	newMarshalable.store = store
	newMarshalable.key = key
	newMarshalable.value = newMarshalable.load()

	return newMarshalable
}

func (m *Marshalable[V, VPtr]) Get() (value V) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.value
}

func (m *Marshalable[V, VPtr]) Set(value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.value = value

	if err := m.store.Set(m.key, lo.PanicOnErr(VPtr(&m.value).Bytes())); err != nil {
		panic(err)
	}
}

func (m *Marshalable[V, VPtr]) load() (loadedValue V) {
	valueBytes, err := m.store.Get(m.key)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return
		}

		panic(err)
	}

	var valuePointer VPtr = new(V)
	if lo.PanicOnErr(valuePointer.FromBytes(valueBytes)) != len(valueBytes) {
		panic("failed to read entire value")
	}

	return *valuePointer
}
