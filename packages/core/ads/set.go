package ads

import (
	"sync"

	"github.com/celestiaorg/smt"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"
	"golang.org/x/crypto/blake2b"
)

const (
	nonEmptyLeaf         = 1
	keyStorePrefix uint8 = iota
	valueStorePrefix
	rawKeyStorePrefix
)

type Set[K constraints.Serializable] struct {
	store kvstore.KVStore
	tree  *smt.SparseMerkleTree

	// A mutex is needed as reads from the smt.SparseMerkleTree can translate to writes.
	mutex sync.Mutex
}

func NewSet[K constraints.Serializable](store kvstore.KVStore) *Set[K] {
	return &Set[K]{
		store: store,
		tree: smt.NewSparseMerkleTree(
			lo.PanicOnErr(store.WithExtendedRealm([]byte{keyStorePrefix})),
			lo.PanicOnErr(store.WithExtendedRealm([]byte{valueStorePrefix})),
			lo.PanicOnErr(blake2b.New256(nil)),
		),
	}
}

// Root returns the root of the state sparse merkle tree at the latest committed epoch.
func (s *Set[K]) Root() (root types.Identifier) {
	if s == nil {
		return types.Identifier{}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	copy(root[:], s.tree.Root())

	return
}

// Add adds the output to unspent outputs set.
func (s *Set[K]) Add(key K) {
	if s == nil {
		panic("cannot add to nil set")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, err := s.tree.Update(lo.PanicOnErr(key.Bytes()), []byte{nonEmptyLeaf}); err != nil {
		panic(err)
	}
}

// Delete removes the output ID from the ledger sparse merkle tree.
func (s *Set[K]) Delete(key K) (deleted bool) {
	if s == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	keyBytes := lo.PanicOnErr(key.Bytes())
	if deleted, _ = s.tree.Has(keyBytes); deleted {
		if _, err := s.tree.Delete(keyBytes); err != nil {
			panic(err)
		}
	}

	return
}

// Has returns true if the key is in the set.
func (s *Set[K]) Has(key K) (has bool) {
	if s == nil {
		return false
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	return lo.PanicOnErr(s.tree.Has(lo.PanicOnErr(key.Bytes())))
}

// Size returns the number of elements in the set.
func (s *Set[K]) Size() (size int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.store.Iterate([]byte{valueStorePrefix}, func(key, value []byte) bool {
		size++
		return true
	})

	return
}
