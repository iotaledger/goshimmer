package blockdag

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework implements a framework for conveniently issuing blocks in a BlockDAG as part of unit tests in a
// simplified way.
type TestFramework struct {
	Instance *BlockDAG

	test                *testing.T
	evictionState       *eviction.State
	solidBlocks         int32
	missingBlocks       int32
	invalidBlocks       int32
	attachedBlocks      int32
	orphanedBlocks      models.BlockIDs
	orphanedBlocksMutex sync.Mutex

	workers    *workerpool.Group
	workerPool *workerpool.UnboundedWorkerPool

	*ModelsTestFramework
}

func NewTestStorage(t *testing.T, workers *workerpool.Group, opts ...options.Option[database.Manager]) *storage.Storage {
	s := storage.New(t.TempDir(), 1, opts...)
	t.Cleanup(func() {
		workers.Wait()
		s.Shutdown()
	})
	return s
}

func NewTestBlockDAG(t *testing.T, workers *workerpool.Group, evictionState *eviction.State, storageInstance *storage.Storage, optsBlockDAG ...options.Option[BlockDAG]) *BlockDAG {
	require.NotNil(t, evictionState)
	return New(workers, evictionState, storageInstance.Commitments.Load, optsBlockDAG...)
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, workers *workerpool.Group, blockDAG *BlockDAG) *TestFramework {
	t := &TestFramework{
		test:           test,
		workers:        workers,
		Instance:       blockDAG,
		orphanedBlocks: models.NewBlockIDs(),
		workerPool:     workers.CreatePool("IssueBlocks", 2),
	}
	t.ModelsTestFramework = models.NewTestFramework(
		models.WithBlock("Genesis", models.NewEmptyBlock(models.EmptyBlockID)),
	)

	t.setupEvents()

	return t
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, optsBlockDAG ...options.Option[BlockDAG]) *TestFramework {
	storageInstance := NewTestStorage(t, workers)
	return NewTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"), NewTestBlockDAG(t, workers.CreateGroup("BlockDAG"), eviction.NewState(storageInstance), storageInstance, optsBlockDAG...))
}

// IssueBlocks stores the given Blocks in the Storage and triggers the processing by the BlockDAG.
func (t *TestFramework) IssueBlocks(blockAliases ...string) *TestFramework {
	for _, alias := range blockAliases {
		currentBlock := t.ModelsTestFramework.Block(alias)

		t.workerPool.Submit(func() {
			_, _, _ = t.Instance.Attach(currentBlock)
		})
	}

	t.workers.WaitAll()

	return t
}

func (t *TestFramework) AssertMissing(expectedValues map[string]bool) {
	for alias, isMissing := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.test, isMissing, block.IsMissing(), "block %s has incorrect missing flag", alias)
		})
	}
}

func (t *TestFramework) AssertInvalid(expectedValues map[string]bool) {
	for alias, isInvalid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.test, isInvalid, block.IsInvalid(), "block %s has incorrect invalid flag", alias)
		})
	}
}

func (t *TestFramework) AssertSolid(expectedValues map[string]bool) {
	for alias, isSolid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.test, isSolid, block.IsSolid(), "block %s has incorrect solid flag", alias)
		})
	}
}

func (t *TestFramework) AssertOrphanedBlocks(orphanedBlocks models.BlockIDs, msgAndArgs ...interface{}) {
	t.orphanedBlocksMutex.Lock()
	defer t.orphanedBlocksMutex.Unlock()

	require.EqualValues(t.test, orphanedBlocks, t.orphanedBlocks, msgAndArgs...)
}

func (t *TestFramework) AssertSolidCount(solidCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.test, solidCount, atomic.LoadInt32(&(t.solidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertInvalidCount(invalidCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.test, invalidCount, atomic.LoadInt32(&(t.invalidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertMissingCount(missingCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.test, missingCount, atomic.LoadInt32(&(t.missingBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertStoredCount(storedCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.test, storedCount, atomic.LoadInt32(&(t.attachedBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertOrphanedCount(storedCount int32, msgAndArgs ...interface{}) {
	t.orphanedBlocksMutex.Lock()
	defer t.orphanedBlocksMutex.Unlock()

	require.EqualValues(t.test, storedCount, len(t.orphanedBlocks), msgAndArgs...)
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Instance.Block(t.Block(alias).ID())
	require.True(t.test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertStrongChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.strongChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertWeakChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.weakChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertLikedInsteadChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.likedInsteadChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) setupEvents() {
	event.Hook(t.Instance.Events.BlockSolid, func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("SOLID: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.solidBlocks), 1)
	})

	event.Hook(t.Instance.Events.BlockMissing, func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("MISSING: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), 1)
	})

	event.Hook(t.Instance.Events.MissingBlockAttached, func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("MISSING BLOCK STORED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), -1)
	})

	event.Hook(t.Instance.Events.BlockInvalid, func(event *BlockInvalidEvent) {
		if debug.GetEnabled() {
			t.test.Logf("INVALID: %s (%s)", event.Block.ID(), event.Reason)
		}
		atomic.AddInt32(&(t.invalidBlocks), 1)
	})

	event.Hook(t.Instance.Events.BlockAttached, func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("ATTACHED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.attachedBlocks), 1)
	})

	event.Hook(t.Instance.Events.BlockOrphaned, func(metadata *Block) {
		t.orphanedBlocksMutex.Lock()
		defer t.orphanedBlocksMutex.Unlock()

		if debug.GetEnabled() {
			t.test.Logf("ORPHANED: %s", metadata.ID())
		}

		t.orphanedBlocks.Add(metadata.ID())
	})

	event.Hook(t.Instance.Events.BlockUnorphaned, func(metadata *Block) {
		t.orphanedBlocksMutex.Lock()
		defer t.orphanedBlocksMutex.Unlock()

		if debug.GetEnabled() {
			t.test.Logf("UNORPHANED: %s", metadata.ID())
		}

		t.orphanedBlocks.Remove(metadata.ID())
	})
}

// ModelsTestFramework is an alias that it is used to be able to embed a named version of the TestFramework.
type ModelsTestFramework = models.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
