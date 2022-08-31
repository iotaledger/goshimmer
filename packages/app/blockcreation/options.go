package blockcreation

import (
	"fmt"

	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
)

func WithStore(store kvstore.KVStore) options.Option[BlockFactory] {
	return func(f *BlockFactory) {
		sequence, err := kvstore.NewSequence(store, []byte(dbSequenceNumber), storeSequenceInterval)
		if err != nil {
			panic(fmt.Sprintf("could not create block sequence number: %v", err))
		}
		f.sequence = sequence
	}
}

func WithIdentity(identity *identity.LocalIdentity) options.Option[BlockFactory] {
	return func(f *BlockFactory) {
		f.identity = identity
	}
}

func WithTipSelector(selector TipSelector) options.Option[BlockFactory] {
	return func(f *BlockFactory) {
		f.selector = selector
	}
}

func WithReferencesFunc(referencesFunc ReferencesFunc) options.Option[BlockFactory] {
	return func(f *BlockFactory) {
		f.referencesFunc = referencesFunc
	}
}

func WithCommitmentFunc(commitmentFunc CommitmentFunc) options.Option[BlockFactory] {
	return func(f *BlockFactory) {
		f.commitmentFunc = commitmentFunc
	}
}
