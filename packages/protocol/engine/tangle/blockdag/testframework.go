package blockdag

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework implements a framework for conveniently issuing blocks in a BlockDAG as part of unit tests in a
// simplified way.
type TestFramework struct {
	Instance BlockDAG

	Test                *testing.T
	evictionState       *eviction.State
	solidBlocks         int32
	missingBlocks       int32
	invalidBlocks       int32
	attachedBlocks      int32
	orphanedBlocks      models.BlockIDs
	orphanedBlocksMutex sync.Mutex

	slotTimeProviderFunc func() *slot.TimeProvider

	workers    *workerpool.Group
	workerPool *workerpool.WorkerPool

	*ModelsTestFramework
}

func NewTestStorage(t *testing.T, workers *workerpool.Group, opts ...options.Option[database.Manager]) *storage.Storage {
	s := storage.New(t.TempDir(), 1, opts...)
	t.Cleanup(func() {
		workers.WaitChildren()
		s.Shutdown()
	})
	return s
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, workers *workerpool.Group, blockDAG BlockDAG, slotTimeProviderFunc func() *slot.TimeProvider) *TestFramework {
	t := &TestFramework{
		Test:                 test,
		workers:              workers,
		Instance:             blockDAG,
		orphanedBlocks:       models.NewBlockIDs(),
		workerPool:           workers.CreatePool("IssueBlocks", 2),
		slotTimeProviderFunc: slotTimeProviderFunc,
	}
	t.ModelsTestFramework = models.NewTestFramework(
		slotTimeProviderFunc,
		models.WithBlock("Genesis", models.NewEmptyBlock(models.EmptyBlockID)),
	)

	t.setupEvents()

	return t
}

func (t *TestFramework) SlotTimeProvider() *slot.TimeProvider {
	return t.slotTimeProviderFunc()
}

// IssueBlocks stores the given Blocks in the Storage and triggers the processing by the BlockDAG.
func (t *TestFramework) IssueBlocks(blockAliases ...string) *TestFramework {
	for _, alias := range blockAliases {
		currentBlock := t.ModelsTestFramework.Block(alias)

		t.workerPool.Submit(func() {
			_, _, _ = t.Instance.Attach(currentBlock)
		})
	}

	t.workers.WaitParents()

	return t
}

func (t *TestFramework) AssertMissing(expectedValues map[string]bool) {
	for alias, isMissing := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.Test, isMissing, block.IsMissing(), "block %s has incorrect missing flag", alias)
		})
	}
}

func (t *TestFramework) AssertInvalid(expectedValues map[string]bool) {
	for alias, isInvalid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.Test, isInvalid, block.IsInvalid(), "block %s has incorrect invalid flag", alias)
		})
	}
}

func (t *TestFramework) AssertSolid(expectedValues map[string]bool) {
	for alias, isSolid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.Test, isSolid, block.IsSolid(), "block %s has incorrect solid flag", alias)
		})
	}
}

func (t *TestFramework) AssertOrphanedBlocks(orphanedBlocks models.BlockIDs, msgAndArgs ...interface{}) {
	t.orphanedBlocksMutex.Lock()
	defer t.orphanedBlocksMutex.Unlock()

	require.EqualValues(t.Test, orphanedBlocks, t.orphanedBlocks, msgAndArgs...)
}

func (t *TestFramework) AssertSolidCount(solidCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.Test, solidCount, atomic.LoadInt32(&(t.solidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertInvalidCount(invalidCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.Test, invalidCount, atomic.LoadInt32(&(t.invalidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertMissingCount(missingCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.Test, missingCount, atomic.LoadInt32(&(t.missingBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertStoredCount(storedCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.Test, storedCount, atomic.LoadInt32(&(t.attachedBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertOrphanedCount(storedCount int32, msgAndArgs ...interface{}) {
	t.orphanedBlocksMutex.Lock()
	defer t.orphanedBlocksMutex.Unlock()

	require.EqualValues(t.Test, storedCount, len(t.orphanedBlocks), msgAndArgs...)
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Instance.Block(t.Block(alias).ID())
	require.True(t.Test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertStrongChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.Test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.strongChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertWeakChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.Test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.weakChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertLikedInsteadChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.Test, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.likedInsteadChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) setupEvents() {
	t.Instance.Events().BlockSolid.Hook(func(metadata *Block) {
		if debug.GetEnabled() {
			t.Test.Logf("SOLID: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.solidBlocks), 1)
	})

	t.Instance.Events().BlockMissing.Hook(func(metadata *Block) {
		if debug.GetEnabled() {
			t.Test.Logf("MISSING: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), 1)
	})

	t.Instance.Events().MissingBlockAttached.Hook(func(metadata *Block) {
		if debug.GetEnabled() {
			t.Test.Logf("MISSING BLOCK STORED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), -1)
	})

	t.Instance.Events().BlockInvalid.Hook(func(event *BlockInvalidEvent) {
		if debug.GetEnabled() {
			t.Test.Logf("INVALID: %s (%s)", event.Block.ID(), event.Reason)
		}
		atomic.AddInt32(&(t.invalidBlocks), 1)
	})

	t.Instance.Events().BlockAttached.Hook(func(metadata *Block) {
		if debug.GetEnabled() {
			t.Test.Logf("ATTACHED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.attachedBlocks), 1)
	})

	t.Instance.Events().BlockOrphaned.Hook(func(metadata *Block) {
		t.orphanedBlocksMutex.Lock()
		defer t.orphanedBlocksMutex.Unlock()

		if debug.GetEnabled() {
			t.Test.Logf("ORPHANED: %s", metadata.ID())
		}

		t.orphanedBlocks.Add(metadata.ID())
	})

	t.Instance.Events().BlockUnorphaned.Hook(func(metadata *Block) {
		t.orphanedBlocksMutex.Lock()
		defer t.orphanedBlocksMutex.Unlock()

		if debug.GetEnabled() {
			t.Test.Logf("UNORPHANED: %s", metadata.ID())
		}

		t.orphanedBlocks.Remove(metadata.ID())
	})
}

func DefaultCommitmentFunc(index slot.Index) (cm *commitment.Commitment, err error) {
	return commitment.New(index, commitment.NewID(1, []byte{}), types.NewIdentifier([]byte{}), 0), nil
}

// ModelsTestFramework is an alias that it is used to be able to embed a named version of the TestFramework.
type ModelsTestFramework = models.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
