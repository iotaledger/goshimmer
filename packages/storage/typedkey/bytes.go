package typedkey

import (
	"sync"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/pkg/errors"
)

type Bytes struct {
	store kvstore.KVStore
	key   []byte
	value []byte
	mutex sync.RWMutex
}

func NewBytes(store kvstore.KVStore, keyBytes ...byte) (newBytes *Bytes) {
	if len(keyBytes) == 0 {
		panic("keyBytes must not be empty")
	}

	newBytes = new(Bytes)
	newBytes.store = store
	newBytes.key = keyBytes
	newBytes.value = newBytes.load()

	return
}

func (b *Bytes) Get() (value []byte) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.value
}

func (b *Bytes) Set(value []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.value = value

	if value == nil {
		if err := b.store.Delete(b.key); err != nil {
			panic(err)
		}
	} else {
		if err := b.store.Set(b.key, value); err != nil {
			panic(err)
		}
	}
}

func (b *Bytes) load() (loadedValue []byte) {
	loadedValue, err := b.store.Get(b.key)
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	}

	return
}
