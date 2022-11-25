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

const (
	nonEmptyLeaf         = 1
	keyStorePrefix uint8 = iota
	valueStorePrefix
	rawKeyStorePrefix
)

type Set[K any, KPtr constraints.MarshalablePtr[K]] struct {
	store   kvstore.KVStore
	tree    *smt.SparseMerkleTree
	rawKeys kvstore.KVStore

	// A mutex is needed as reads from the smt.SparseMerkleTree can translate to writes.
	mutex sync.Mutex
}

func NewSet[K any, KPtr constraints.MarshalablePtr[K]](store kvstore.KVStore) *Set[K, KPtr] {
	return &Set[K, KPtr]{
		store: store,
		tree: smt.NewSparseMerkleTree(
			lo.PanicOnErr(store.WithExtendedRealm([]byte{keyStorePrefix})),
			lo.PanicOnErr(store.WithExtendedRealm([]byte{valueStorePrefix})),
			lo.PanicOnErr(blake2b.New256(nil)),
		),
		rawKeys: lo.PanicOnErr(store.WithExtendedRealm([]byte{rawKeyStorePrefix})),
	}
}

// Root returns the root of the state sparse merkle tree at the latest committed epoch.
func (s *Set[K, KPtr]) Root() (root types.Identifier) {
	if s == nil {
		return types.Identifier{}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	copy(root[:], s.tree.Root())

	return
}

// Add adds the output to unspent outputs set.
func (s *Set[K, KPtr]) Add(key K) {
	if s == nil {
		panic("cannot add to nil set")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, err := s.tree.Update(lo.PanicOnErr(KPtr(&key).Bytes()), []byte{nonEmptyLeaf}); err != nil {
		panic(err)
	}

	if err := s.rawKeys.Set(lo.PanicOnErr(KPtr(&key).Bytes()), []byte{}); err != nil {
		panic(err)
	}
}

// Delete removes the output ID from the ledger sparse merkle tree.
func (s *Set[K, KPtr]) Delete(key K) (deleted bool) {
	if s == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	keyBytes := lo.PanicOnErr(KPtr(&key).Bytes())
	if deleted, _ = s.tree.Has(keyBytes); deleted {
		if _, err := s.tree.Delete(keyBytes); err != nil {
			panic(err)
		}

		if err := s.rawKeys.Delete(lo.PanicOnErr(KPtr(&key).Bytes())); err != nil {
			panic(err)
		}
	}

	return
}

// Has returns true if the key is in the set.
func (s *Set[K, KPtr]) Has(key K) (has bool) {
	if s == nil {
		return false
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	return lo.PanicOnErr(s.tree.Has(lo.PanicOnErr(KPtr(&key).Bytes())))
}

func (s *Set[K, KPtr]) Stream(callback func(key K) bool) (err error) {
	if s == nil {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if iterationErr := s.rawKeys.Iterate([]byte{}, func(key kvstore.Key, _ kvstore.Value) bool {
		var kPtr KPtr = new(K)
		if _, keyErr := kPtr.FromBytes(key); keyErr != nil {
			err = errors.Errorf("failed to deserialize key %s: %w", key, keyErr)
			return false
		}

		return callback(*kPtr)
	}); iterationErr != nil {
		err = errors.Errorf("failed to iterate over set members: %w", iterationErr)
	}

	return
}

// Size returns the number of elements in the set.
func (s *Set[K, KPtr]) Size() (size int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.store.Iterate([]byte{valueStorePrefix}, func(key, value []byte) bool {
		size++
		return true
	})

	return
}
