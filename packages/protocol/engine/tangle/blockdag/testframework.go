package blockdag

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework implements a framework for conveniently issuing blocks in a BlockDAG as part of unit tests in a
// simplified way.
type TestFramework struct {
	BlockDAG *BlockDAG

	test                *testing.T
	evictionManager     *eviction.State[models.BlockID]
	solidBlocks         int32
	missingBlocks       int32
	invalidBlocks       int32
	attachedBlocks      int32
	orphanedBlocks      models.BlockIDs
	orphanedBlocksMutex sync.Mutex

	optsBlockDAG []options.Option[BlockDAG]

	*ModelsTestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:           test,
		orphanedBlocks: models.NewBlockIDs(),
	}, opts, func(t *TestFramework) {
		if t.BlockDAG == nil {
			if t.evictionManager == nil {
				t.evictionManager = eviction.NewState[models.BlockID]()
			}

			t.BlockDAG = New(t.evictionManager, t.optsBlockDAG...)
		}

		t.ModelsTestFramework = models.NewTestFramework(
			models.WithBlock("Genesis", models.NewEmptyBlock(models.EmptyBlockID)),
		)
	}, (*TestFramework).setupEvents)
}

// IssueBlocks stores the given Blocks in the Storage and triggers the processing by the BlockDAG.
func (t *TestFramework) IssueBlocks(blockAliases ...string) *TestFramework {
	for _, alias := range blockAliases {
		currentBlock := t.ModelsTestFramework.Block(alias)

		event.Loop.Submit(func() {
			_, _, _ = t.BlockDAG.Attach(currentBlock)
		})
	}

	return t
}

// WaitUntilAllTasksProcessed waits until all tasks are processed.
func (t *TestFramework) WaitUntilAllTasksProcessed() (self *TestFramework) {
	event.Loop.WaitUntilAllTasksProcessed()
	return t
}

func (t *TestFramework) AssertMissing(expectedValues map[string]bool) {
	for alias, isMissing := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.test, isMissing, block.IsMissing(), "block %s has incorrect missing flag", alias)
		})
	}
}

func (t *TestFramework) AssertInvalid(expectedValues map[string]bool) {
	for alias, isInvalid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.test, isInvalid, block.IsInvalid(), "block %s has incorrect invalid flag", alias)
		})
	}
}

func (t *TestFramework) AssertSolid(expectedValues map[string]bool) {
	for alias, isSolid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.test, isSolid, block.IsSolid(), "block %s has incorrect solid flag", alias)
		})
	}
}

func (t *TestFramework) AssertOrphanedBlocks(orphanedBlocks models.BlockIDs, msgAndArgs ...interface{}) {
	t.orphanedBlocksMutex.Lock()
	defer t.orphanedBlocksMutex.Unlock()

	assert.EqualValues(t.test, orphanedBlocks, t.orphanedBlocks, msgAndArgs...)
}

func (t *TestFramework) AssertSolidCount(solidCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.test, solidCount, atomic.LoadInt32(&(t.solidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertInvalidCount(invalidCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.test, invalidCount, atomic.LoadInt32(&(t.invalidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertMissingCount(missingCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.test, missingCount, atomic.LoadInt32(&(t.missingBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertStoredCount(storedCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.test, storedCount, atomic.LoadInt32(&(t.attachedBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertOrphanedCount(storedCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.test, storedCount, len(t.orphanedBlocks), msgAndArgs...)
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.BlockDAG.Block(t.Block(alias).ID())
	require.True(t.test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertStrongChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.strongChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertWeakChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.weakChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertLikedInsteadChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.likedInsteadChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) setupEvents() {
	t.BlockDAG.Events.BlockSolid.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("SOLID: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.solidBlocks), 1)
	}))

	t.BlockDAG.Events.BlockMissing.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("MISSING: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), 1)
	}))

	t.BlockDAG.Events.MissingBlockAttached.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("MISSING BLOCK STORED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), -1)
	}))

	t.BlockDAG.Events.BlockInvalid.Hook(event.NewClosure(func(event *BlockInvalidEvent) {
		if debug.GetEnabled() {
			t.test.Logf("INVALID: %s (%s)", event.Block.ID(), event.Reason)
		}
		atomic.AddInt32(&(t.invalidBlocks), 1)
	}))

	t.BlockDAG.Events.BlockAttached.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("ATTACHED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.attachedBlocks), 1)
	}))

	t.BlockDAG.Events.BlockOrphaned.Hook(event.NewClosure(func(metadata *Block) {
		t.orphanedBlocksMutex.Lock()
		defer t.orphanedBlocksMutex.Unlock()

		if debug.GetEnabled() {
			t.test.Logf("ORPHANED: %s", metadata.ID())
		}

		t.orphanedBlocks.Add(metadata.ID())
	}))

	t.BlockDAG.Events.BlockUnorphaned.Hook(event.NewClosure(func(metadata *Block) {
		t.orphanedBlocksMutex.Lock()
		defer t.orphanedBlocksMutex.Unlock()

		if debug.GetEnabled() {
			t.test.Logf("UNORPHANED: %s", metadata.ID())
		}

		t.orphanedBlocks.Remove(metadata.ID())
	}))
}

type ModelsTestFramework = models.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEvictionManager(evictionManager *eviction.State[models.BlockID]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.evictionManager = evictionManager
	}
}

func WithBlockDAGOptions(opts ...options.Option[BlockDAG]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBlockDAG = opts
	}
}

func WithBlockDAG(blockDAG *BlockDAG) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.BlockDAG = blockDAG
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
