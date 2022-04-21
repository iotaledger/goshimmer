package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm/indexer"
)

var (
	index *indexer.Indexer
)

// Indexer is the indexer instance.
func Indexer() *indexer.Indexer {
	return index
}
