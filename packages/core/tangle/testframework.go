package tangle

import (
	"testing"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework implements a framework for conveniently issuing blocks in a tangle as part of unit tests in a
// simplified way.
type TestFramework struct {
	Tangle       *Tangle
	genesisBlock *Block

	t *testing.T
	*models.TestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(testingT *testing.T, opts ...options.Option[Tangle]) (t *TestFramework) {
	t = &TestFramework{
		genesisBlock: NewBlock(models.NewEmptyBlock(models.EmptyBlockID), WithSolid(true)),
	}
	t.TestFramework = models.NewTestFramework(models.WithBlock("Genesis", t.genesisBlock.Block))
	t.Tangle = New(database.NewManager(testingT.TempDir(), database.WithDBProvider(mapdb.NewMapDB)), t.rootBlockProvider, opts...)
	t.t = testingT

	return
}

// IssueBlocks stores the given Blocks in the Storage and triggers the processing by the Tangle.
func (t *TestFramework) IssueBlocks(blockAliases ...string) *TestFramework {
	for _, alias := range blockAliases {
		currentBlock := t.Block(alias)

		event.Loop.Submit(func() {
			_, _, _ = t.Tangle.Attach(currentBlock)
		})
	}

	return t
}

// WaitUntilAllTasksProcessed waits until all tasks are processed.
func (t *TestFramework) WaitUntilAllTasksProcessed() (self *TestFramework) {
	// time.Sleep(100 * time.Millisecond)
	event.Loop.WaitUntilAllTasksProcessed()
	return t
}

func (t *TestFramework) Shutdown() {
	t.Tangle.Shutdown()
}

func (t *TestFramework) AssertMissing(expectedValues map[string]bool) {
	for alias, isMissing := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.t, isMissing, block.IsMissing())
		})
	}
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Tangle.Block(t.Block(alias).ID())
	assert.True(t.t, exists, "Block %s not found", alias)
	callback(block)
}

// rootBlockProvider is a default function that determines whether a block is a root of the Tangle.
func (t *TestFramework) rootBlockProvider(blockID models.BlockID) (block *Block) {
	if blockID != t.genesisBlock.ID() {
		return
	}

	return t.genesisBlock
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
