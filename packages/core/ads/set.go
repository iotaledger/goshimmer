package ads

import (
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
)

type Set[K constraints.Serializable] struct {
	tree *smt.SparseMerkleTree
}

func NewSet[K constraints.Serializable](store kvstore.KVStore) *Set[K] {
	return &Set[K]{
		tree: smt.NewSparseMerkleTree(
			lo.PanicOnErr(store.WithExtendedRealm([]byte{keyStorePrefix})),
			lo.PanicOnErr(store.WithExtendedRealm([]byte{valueStorePrefix})),
			lo.PanicOnErr(blake2b.New256(nil)),
		),
	}
}

// Root returns the root of the state sparse merkle tree at the latest committed epoch.
func (s *Set[K]) Root() (root types.Identifier) {
	copy(root[:], s.tree.Root())

	return
}

// Add adds the output to unspent outputs set.
func (s *Set[K]) Add(key K) {
	if _, err := s.tree.Update(lo.PanicOnErr(key.Bytes()), []byte{nonEmptyLeaf}); err != nil {
		panic(err)
	}
}

// Delete removes the output ID from the ledger sparse merkle tree.
func (s *Set[K]) Delete(key K) {
	keyBytes := lo.PanicOnErr(key.Bytes())
	if exists, _ := s.tree.Has(keyBytes); exists {
		if _, err := s.tree.Delete(keyBytes); err != nil {
			panic(err)
		}
	}
}

// Has returns true if the key is in the set.
func (s *Set[K]) Has(key K) (has bool) {
	return lo.PanicOnErr(s.tree.Has(lo.PanicOnErr(key.Bytes())))
}
