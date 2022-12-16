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

const (
	PrefixRawKeysStorage uint8 = iota
	PrefixSMTKeysStorage
	PrefixSMTValuesStorage
	PrefixRootKey
	PrefixSizeKey

	nonEmptyLeaf = 1
)

// Set is a sparse merkle tree based set.
type Set[K any, KPtr constraints.MarshalablePtr[K]] struct {
	rawKeysStore kvstore.KVStore
	tree         *smt.SparseMerkleTree
	root         *typedkey.Bytes
	size         *typedkey.Number[uint64]
	mutex        sync.Mutex
}

// NewSet creates a new sparse merkle tree based set.
func NewSet[K any, KPtr constraints.MarshalablePtr[K]](store kvstore.KVStore) (newSet *Set[K, KPtr]) {
	newSet = &Set[K, KPtr]{
		rawKeysStore: lo.PanicOnErr(store.WithExtendedRealm([]byte{PrefixRawKeysStorage})),
		tree: smt.NewSparseMerkleTree(
			lo.PanicOnErr(store.WithExtendedRealm([]byte{PrefixSMTKeysStorage})),
			lo.PanicOnErr(store.WithExtendedRealm([]byte{PrefixSMTValuesStorage})),
			lo.PanicOnErr(blake2b.New256(nil)),
		),
		root: typedkey.NewBytes(store, PrefixRootKey),
		size: typedkey.NewNumber[uint64](store, PrefixSizeKey),
	}

	if root := newSet.root.Get(); len(root) != 0 {
		newSet.tree.SetRoot(root)
	}

	return
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

// Add adds the key to the set.
func (s *Set[K, KPtr]) Add(key K) {
	if s == nil {
		panic("cannot add to nil set")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	keyBytes := lo.PanicOnErr(KPtr(&key).Bytes())
	if s.has(keyBytes) {
		return
	}

	s.root.Set(lo.PanicOnErr(s.tree.Update(keyBytes, []byte{nonEmptyLeaf})))
	s.size.Inc()

	if err := s.rawKeysStore.Set(keyBytes, []byte{}); err != nil {
		panic(err)
	}
}

// Delete removes the key from the set.
func (s *Set[K, KPtr]) Delete(key K) (deleted bool) {
	if s == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	keyBytes := lo.PanicOnErr(KPtr(&key).Bytes())
	if deleted = s.has(keyBytes); !deleted {
		return
	}

	s.root.Set(lo.PanicOnErr(s.tree.Delete(keyBytes)))
	s.size.Dec()

	if err := s.rawKeysStore.Delete(keyBytes); err != nil {
		panic(err)
	}

	return
}

// Has returns true if the key is in the set.
func (s *Set[K, KPtr]) Has(key K) (has bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.has(lo.PanicOnErr(KPtr(&key).Bytes()))
}

// Stream iterates over the set and calls the callback for each element.
func (s *Set[K, KPtr]) Stream(callback func(key K) bool) (err error) {
	if s == nil {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if iterationErr := s.rawKeysStore.Iterate([]byte{}, func(key kvstore.Key, _ kvstore.Value) bool {
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
	if s == nil {
		return 0
	}

	return int(s.size.Get())
}

// has returns true if the key is in the set.
func (s *Set[K, KPtr]) has(key []byte) (has bool) {
	if s == nil {
		return false
	}

	has, err := s.tree.Has(key)
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return false
		}

		panic(err)
	}

	return
}
