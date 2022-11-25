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
)

type Map[K, V constraints.Serializable, KPtr constraints.MarshalablePtr[K], VPtr constraints.MarshalablePtr[V]] struct {
	tree    *smt.SparseMerkleTree
	rawKeys kvstore.KVStore

	// A mutex is needed as reads from the smt.SparseMerkleTree can translate to writes.
	mutex sync.Mutex
}

func NewMap[K, V constraints.Serializable, KPtr constraints.MarshalablePtr[K], VPtr constraints.MarshalablePtr[V]](store kvstore.KVStore) *Map[K, V, KPtr, VPtr] {
	return &Map[K, V, KPtr, VPtr]{
		tree: smt.NewSparseMerkleTree(
			lo.PanicOnErr(store.WithExtendedRealm([]byte{keyStorePrefix})),
			lo.PanicOnErr(store.WithExtendedRealm([]byte{valueStorePrefix})),
			lo.PanicOnErr(blake2b.New256(nil)),
		),
		rawKeys: lo.PanicOnErr(store.WithExtendedRealm([]byte{rawKeyStorePrefix})),
	}
}

// Root returns the root of the state sparse merkle tree at the latest committed epoch.
func (m *Map[K, V, KPtr, VPtr]) Root() (root types.Identifier) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	copy(root[:], m.tree.Root())

	return
}

// Set sets the output to unspent outputs set.
func (m *Map[K, V, KPtr, VPtr]) Set(key K, value VPtr) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	valueBytes := lo.PanicOnErr(value.Bytes())
	if len(valueBytes) == 0 {
		panic("value cannot be empty")
	}

	if _, err := m.tree.Update(lo.PanicOnErr(key.Bytes()), valueBytes); err != nil {
		panic(err)
	}

	if err := m.rawKeys.Set(lo.PanicOnErr(key.Bytes()), []byte{}); err != nil {
		panic(err)
	}
}

// Delete removes the output ID from the ledger sparse merkle tree.
func (m *Map[K, V, KPtr, VPtr]) Delete(key K) (deleted bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	keyBytes := lo.PanicOnErr(key.Bytes())
	if deleted, _ = m.tree.Has(keyBytes); deleted {
		if _, err := m.tree.Delete(keyBytes); err != nil {
			panic(err)
		}

		if err := m.rawKeys.Delete(lo.PanicOnErr(key.Bytes())); err != nil {
			panic(err)
		}
	}

	return
}

// Has returns true if the key is in the set.
func (m *Map[K, V, KPtr, VPtr]) Has(key K) (has bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return lo.PanicOnErr(m.tree.Has(lo.PanicOnErr(key.Bytes())))
}

// Get returns the value for the given key.
func (m *Map[K, V, KPtr, VPtr]) Get(key K) (value VPtr, exists bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	valueBytes, err := m.tree.Get(lo.PanicOnErr(key.Bytes()))
	if errors.Is(err, kvstore.ErrKeyNotFound) {
		return nil, false
	}
	if err != nil {
		panic(err)
	}
	if len(valueBytes) == 0 {
		return nil, false
	}

	value = new(V)
	if _, err := value.FromBytes(valueBytes); err != nil {
		panic(err)
	}

	return value, true
}

func (m *Map[K, V, KPtr, VPtr]) Stream(callback func(key K, value VPtr) bool) (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if iterationErr := m.rawKeys.Iterate([]byte{}, func(key kvstore.Key, _ kvstore.Value) bool {
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
