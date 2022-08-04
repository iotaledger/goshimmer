package booker

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

type TestFramework struct {
	Booker       *Booker
	genesisBlock *Block

	*tangle.TestFramework
	ledgerTf *ledger.TestFramework
}

func NewTestFramework(t *testing.T) (newTestFramework *TestFramework) {
	newTestFramework = &TestFramework{
		TestFramework: tangle.NewTestFramework(t),
		ledgerTf:      ledger.NewTestFramework(t),
		genesisBlock:  NewBlock(tangle.NewBlock(models.NewEmptyBlock(models.EmptyBlockID), tangle.WithSolid(true)), WithBooked(true)),
	}
	newTestFramework.Booker = New(newTestFramework.Tangle, newTestFramework.ledgerTf.Ledger(), newTestFramework.rootBlockProvider)

	return
}

// rootBlockProvider is a default function that determines whether a block is a root of the Tangle.
func (t *TestFramework) rootBlockProvider(blockID models.BlockID) (block *Block) {
	if blockID != t.genesisBlock.ID() {
		return
	}

	return t.genesisBlock
}
