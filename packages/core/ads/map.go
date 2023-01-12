package ads

import (
	"sync"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/storage/typedkey"
)

type Map[K, V constraints.Serializable, KPtr constraints.MarshalablePtr[K], VPtr constraints.MarshalablePtr[V]] struct {
	rawKeysStore kvstore.KVStore
	tree         *smt.SparseMerkleTree
	root         *typedkey.Bytes
	mutex        sync.RWMutex
}

func NewMap[K, V constraints.Serializable, KPtr constraints.MarshalablePtr[K], VPtr constraints.MarshalablePtr[V]](store kvstore.KVStore) (newMap *Map[K, V, KPtr, VPtr]) {
	newMap = &Map[K, V, KPtr, VPtr]{
		rawKeysStore: lo.PanicOnErr(store.WithExtendedRealm([]byte{PrefixRawKeysStorage})),
		tree: smt.NewSparseMerkleTree(
			lo.PanicOnErr(store.WithExtendedRealm([]byte{PrefixSMTKeysStorage})),
			lo.PanicOnErr(store.WithExtendedRealm([]byte{PrefixSMTValuesStorage})),
			lo.PanicOnErr(blake2b.New256(nil)),
		),
		root: typedkey.NewBytes(store, PrefixRootKey),
	}

	if root := newMap.root.Get(); len(root) != 0 {
		newMap.tree.SetRoot(root)
	}

	return
}

// Root returns the root of the state sparse merkle tree at the latest committed epoch.
func (m *Map[K, V, KPtr, VPtr]) Root() (root types.Identifier) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	copy(root[:], m.tree.Root())

	return
}

// Set sets the output to unspent outputs set.
func (m *Map[K, V, KPtr, VPtr]) Set(key K, value VPtr) {
	valueBytes := lo.PanicOnErr(value.Bytes())
	if len(valueBytes) == 0 {
		panic("value cannot be empty")
	}

	m.set(lo.PanicOnErr(key.Bytes()), valueBytes)
}

func (m *Map[K, V, KPtr, VPtr]) set(key, value []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.root.Set(lo.PanicOnErr(m.tree.Update(key, value)))

	if err := m.rawKeysStore.Set(key, []byte{}); err != nil {
		panic(err)
	}
}

// Delete removes the key from the map.
func (m *Map[K, V, KPtr, VPtr]) Delete(key K) (deleted bool) {
	if m == nil {
		return
	}

	keyBytes := lo.PanicOnErr(key.Bytes())
	if deleted = m.has(keyBytes); deleted {
		m.delete(keyBytes)
	}

	return
}

func (m *Map[K, V, KPtr, VPtr]) delete(keyBytes []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.root.Set(lo.PanicOnErr(m.tree.Delete(keyBytes)))

	if err := m.rawKeysStore.Delete(keyBytes); err != nil {
		panic(err)
	}

	return
}

// Has returns true if the key is in the set.
func (m *Map[K, V, KPtr, VPtr]) Has(key K) (has bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.has(lo.PanicOnErr(key.Bytes()))
}

// Get returns the value for the given key.
func (m *Map[K, V, KPtr, VPtr]) Get(key K) (value VPtr, exists bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	valueBytes, err := m.tree.Get(lo.PanicOnErr(key.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, false
		}

		panic(err)
	}

	if len(valueBytes) == 0 {
		return nil, false
	}

	value = new(V)
	if lo.PanicOnErr(value.FromBytes(valueBytes)) != len(valueBytes) {
		panic("failed to parse entire value")
	}

	return value, true
}

// Stream streams all the keys and values.
func (m *Map[K, V, KPtr, VPtr]) Stream(callback func(key K, value VPtr) bool) (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if iterationErr := m.rawKeysStore.Iterate([]byte{}, func(key kvstore.Key, _ kvstore.Value) bool {
		value, valueErr := m.tree.Get(key)
		if valueErr != nil {
			err = errors.Errorf("failed to get value for key %s: %w", key, valueErr)
			return false
		}

		var kPtr KPtr = new(K)
		if _, keyErr := kPtr.FromBytes(key); keyErr != nil {
			err = errors.Errorf("failed to deserialize key %s: %w", key, keyErr)
			return false
		}

		var valuePtr VPtr = new(V)
		if _, valueErr := valuePtr.FromBytes(value); valueErr != nil {
			err = errors.Errorf("failed to deserialize value %s: %w", value, valueErr)
			return false
		}

		return callback(*kPtr, valuePtr)
	}); iterationErr != nil {
		err = errors.Errorf("failed to iterate over raw keys: %w", iterationErr)
	}

	return
}

// has returns true if the key is in the map.
func (m *Map[K, V, KPtr, VPtr]) has(keyBytes []byte) (has bool) {
	if m == nil {
		return false
	}

	has, err := m.tree.Has(keyBytes)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return false
		}

		panic(err)
	}

	return
}
