package tangle

import (
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework implements a framework for conveniently issuing blocks in a tangle as part of unit tests in a
// simplified way.
type TestFramework struct {
	T               *testing.T
	EvictionManager *eviction.Manager
	Tangle          *Tangle

	solidBlocks    int32
	missingBlocks  int32
	invalidBlocks  int32
	attachedBlocks int32

	optsEvictionManager *eviction.Manager
	optsTangle          []options.Option[Tangle]

	*models.TestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(testingT *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	t = options.Apply(&TestFramework{
		T:             testingT,
		TestFramework: models.NewTestFramework(models.WithBlock("Genesis", models.EmptyBlock)),
	}, opts)

	if t.EvictionManager == nil {
		t.EvictionManager = eviction.NewManager(models.IsEmptyBlockID)
	}

	if t.Tangle == nil {
		t.Tangle = New(t.EvictionManager, t.optsTangle...)
	}

	t.Setup()

	return
}

func (t *TestFramework) Setup() {
	t.Tangle.Events.BlockSolid.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.T.Logf("SOLID: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.solidBlocks), 1)
	}))

	t.Tangle.Events.BlockMissing.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.T.Logf("MISSING: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), 1)
	}))

	t.Tangle.Events.MissingBlockAttached.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.T.Logf("MISSING BLOCK STORED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.missingBlocks), -1)
	}))

	t.Tangle.Events.BlockInvalid.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.T.Logf("INVALID: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.invalidBlocks), 1)
	}))

	t.Tangle.Events.BlockAttached.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.T.Logf("ATTACHED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.attachedBlocks), 1)
	}))
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

func (t *TestFramework) AssertMissing(expectedValues map[string]bool) {
	for alias, isMissing := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, isMissing, block.IsMissing(), "block %s has incorrect missing flag", alias)
		})
	}
}

func (t *TestFramework) AssertInvalid(expectedValues map[string]bool) {
	for alias, isInvalid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, isInvalid, block.IsInvalid(), "block %s has incorrect invalid flag", alias)
		})
	}
}

func (t *TestFramework) AssertSolid(expectedValues map[string]bool) {
	for alias, isSolid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, isSolid, block.IsSolid(), "block %s has incorrect solid flag", alias)
		})
	}
}

func (t *TestFramework) AssertSolidCount(solidCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.T, solidCount, atomic.LoadInt32(&(t.solidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertInvalidCount(invalidCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.T, invalidCount, atomic.LoadInt32(&(t.invalidBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertMissingCount(missingCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.T, missingCount, atomic.LoadInt32(&(t.missingBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertStoredCount(storedCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.T, storedCount, atomic.LoadInt32(&(t.attachedBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Tangle.Block(t.Block(alias).ID())
	require.True(t.T, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertStrongChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.strongChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertWeakChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.weakChildren, (*Block).ID)...))
		})
	}
}

func (t *TestFramework) AssertLikedInsteadChildren(m map[string][]string) {
	for alias, children := range m {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, t.BlockIDs(children...), models.NewBlockIDs(lo.Map(block.likedInsteadChildren, (*Block).ID)...))
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEvictionManager(evictionManager *eviction.Manager) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.EvictionManager = evictionManager
	}
}

func WithTangle(tangle *Tangle) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.Tangle = tangle
	}
}

func WithTangleOptions(opts ...options.Option[Tangle]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsTangle = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
