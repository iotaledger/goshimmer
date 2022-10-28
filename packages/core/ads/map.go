package ads

import (
	"github.com/celestiaorg/smt"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"
	"golang.org/x/crypto/blake2b"
)

type Map[K, V constraints.Serializable, VPtr constraints.MarshalablePtr[V]] struct {
	tree *smt.SparseMerkleTree
}

func NewMap[K, V constraints.Serializable, VPtr constraints.MarshalablePtr[V]](store kvstore.KVStore) *Map[K, V, VPtr] {
	return &Map[K, V, VPtr]{
		tree: smt.NewSparseMerkleTree(
			lo.PanicOnErr(store.WithExtendedRealm([]byte{keyStorePrefix})),
			lo.PanicOnErr(store.WithExtendedRealm([]byte{valueStorePrefix})),
			lo.PanicOnErr(blake2b.New256(nil)),
		),
	}
}

// Root returns the root of the state sparse merkle tree at the latest committed epoch.
func (m *Map[K, V, VPtr]) Root() (root types.Identifier) {
	copy(root[:], m.tree.Root())

	return
}

// Set sets the output to unspent outputs set.
func (m *Map[K, V, VPtr]) Set(key K, value VPtr) {
	valueBytes := lo.PanicOnErr(value.Bytes())
	if len(valueBytes) == 0 {
		panic("value cannot be empty")
	}
	if _, err := m.tree.Update(lo.PanicOnErr(key.Bytes()), valueBytes); err != nil {
		panic(err)
	}
}

// Delete removes the output ID from the ledger sparse merkle tree.
func (m *Map[K, V, VPtr]) Delete(key K) (deleted bool) {
	keyBytes := lo.PanicOnErr(key.Bytes())
	if deleted, _ = m.tree.Has(keyBytes); deleted {
		if _, err := m.tree.Delete(keyBytes); err != nil {
			panic(err)
		}
	}

	return
}

// Has returns true if the key is in the set.
func (m *Map[K, V, VPtr]) Has(key K) (has bool) {
	return lo.PanicOnErr(m.tree.Has(lo.PanicOnErr(key.Bytes())))
}

// Get returns the value for the given key.
func (m *Map[K, V, VPtr]) Get(key K) (value VPtr, exists bool) {
	valueBytes := lo.PanicOnErr(m.tree.Get(lo.PanicOnErr(key.Bytes())))
	if len(valueBytes) == 0 {
		return value, false
	}

	value = new(V)
	if _, err := value.FromBytes(valueBytes); err != nil {
		panic(err)
	}

	return value, true
}
