package inmemoryblockdag

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/randommap"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// This test checks if the internal metadata is correct i.e. that children are assigned correctly and that all the flags are correct.
func TestBlockDAG_AttachBlock(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))
	tf.CreateBlock("block3", models.WithStrongParents(tf.BlockIDs("block1", "block2")))
	tf.CreateBlock("block4", models.WithStrongParents(tf.BlockIDs("block3", "block2")))
	tf.CreateBlock("block5", models.WithStrongParents(tf.BlockIDs("block3", "block4")))

	// issue block2
	{
		tf.IssueBlocks("block2")
		workers.WaitChildren()
		tf.AssertMissing(map[string]bool{
			"block1": true,
			"block2": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {},
		})

		tf.AssertBlock("block1", func(block *blockdag.Block) {
			require.True(t, block.ModelsBlock.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block1
	{
		tf.IssueBlocks("block1")
		workers.WaitChildren()

		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {},
		})

		tf.AssertBlock("block1", func(block *blockdag.Block) {
			require.False(t, block.ModelsBlock.IssuingTime().IsZero(), "block should be attached")
		})
	}

	// issue block4
	{
		tf.IssueBlocks("block4")
		workers.WaitChildren()

		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": true,
			"block4": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
			"block3": false,
			"block4": false,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {"block4"},
			"block3": {"block4"},
			"block4": {},
		})

		tf.AssertBlock("block3", func(block *blockdag.Block) {
			require.True(t, block.ModelsBlock.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block5
	{
		tf.IssueBlocks("block5")
		workers.WaitChildren()

		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": true,
			"block4": false,
			"block5": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {"block4"},
			"block3": {"block4", "block5"},
			"block4": {"block5"},
			"block5": {},
		})

		tf.AssertBlock("block3", func(block *blockdag.Block) {
			require.True(t, block.ModelsBlock.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block3
	{
		tf.IssueBlocks("block3")
		workers.WaitChildren()

		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
			"block3": true,
			"block4": true,
			"block5": true,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2", "block3"},
			"block2": {"block4", "block3"},
			"block3": {"block4", "block5"},
			"block4": {"block5"},
			"block5": {},
		})

		tf.AssertBlock("block3", func(block *blockdag.Block) {
			require.False(t, block.ModelsBlock.IssuingTime().IsZero(), "block should be attached")
		})
	}
}

func TestBlockDAG_SetOrphaned(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	tf.CreateBlock("block1")
	tf.CreateBlock("block2")
	tf.CreateBlock("block3", models.WithStrongParents(tf.BlockIDs("block1", "block2")))
	tf.CreateBlock("block4", models.WithStrongParents(tf.BlockIDs("block3")))
	tf.CreateBlock("block5", models.WithStrongParents(tf.BlockIDs("block4")))
	tf.CreateBlock("block6", models.WithStrongParents(tf.BlockIDs("block5")))
	tf.IssueBlocks("block1", "block2", "block3", "block4", "block5")
	workers.WaitChildren()

	block1, _ := tf.Instance.Block(tf.Block("block1").ID())
	block2, _ := tf.Instance.Block(tf.Block("block2").ID())
	block4, _ := tf.Instance.Block(tf.Block("block4").ID())

	tf.Instance.SetOrphaned(block1, true)
	workers.WaitChildren()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block1"))

	tf.Instance.SetOrphaned(block2, true)
	workers.WaitChildren()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block1", "block2"))

	tf.Instance.SetOrphaned(block4, true)
	workers.WaitChildren()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block1", "block2", "block4"))

	tf.Instance.SetOrphaned(block1, false)
	workers.WaitChildren()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block2", "block4"))

	tf.Instance.SetOrphaned(block2, false)
	workers.WaitChildren()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block4"))

	tf.IssueBlocks("block6")
	workers.WaitChildren()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block4"))

	tf.Instance.SetOrphaned(block4, false)
	workers.WaitChildren()
	tf.AssertOrphanedBlocks(models.NewBlockIDs())
}

func TestBlockDAG_AttachBlockTwice_1(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))

	var (
		wasAttached1 bool
		wasAttached2 bool
		err1         error
		err2         error
		started      uint8
		startMutex   sync.RWMutex
	)

	loop := workers.CreatePool("Loop", 2)

	loop.Submit(func() {
		startMutex.Lock()
		started++
		startMutex.Unlock()

		_, wasAttached1, err1 = tf.Instance.Attach(tf.Block("block2"))
	})
	loop.Submit(func() {
		startMutex.Lock()
		started++
		startMutex.Unlock()

		_, wasAttached2, err2 = tf.Instance.Attach(tf.Block("block2"))
	})

	workers.WaitChildren()

	require.Eventually(t, func() bool {
		startMutex.RLock()
		defer startMutex.RUnlock()

		return started == 2
	}, time.Second*10, time.Millisecond*10)

	require.NoError(t, err1, "should not return an error")
	require.NoError(t, err2, "should not return an error")
	require.True(t, wasAttached1 != wasAttached2, "only one of the two should have been attached")
}

func TestBlockDAG_AttachBlockTwice_2(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))

	_, wasAttached, err := tf.Instance.Attach(tf.Block("block2"))
	require.NoError(t, err, "should not return an error")
	require.True(t, wasAttached, "should have been attached")

	_, wasAttached, err = tf.Instance.Attach(tf.Block("block2"))
	require.NoError(t, err, "should not return an error")
	require.False(t, wasAttached, "should not have been attached")

	workers.WaitChildren()

	require.NoError(t, err, "should not return an error")
}

func TestBlockDAG_Attach_InvalidTimestamp(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	now := tf.Instance.(*BlockDAG).SlotTimeProvider().StartTime(10)
	tf.CreateBlock("block1", models.WithIssuingTime(now.Add(-5*time.Second)))
	tf.CreateBlock("block2", models.WithIssuingTime(now.Add(5*time.Second)))
	tf.CreateBlock("block3", models.WithStrongParents(tf.BlockIDs("block1", "block2")), models.WithIssuingTime(now))

	_, wasAttached, err := tf.Instance.Attach(tf.Block("block1"))
	require.NoError(t, err, "should not return an error")
	require.True(t, wasAttached, "should have been attached")

	_, wasAttached, err = tf.Instance.Attach(tf.Block("block2"))
	require.NoError(t, err, "should not return an error")
	require.True(t, wasAttached, "should have been attached")

	workers.WaitChildren()

	expectedSolidState := map[string]bool{}
	expectedInvalidState := map[string]bool{}

	tf.AssertSolid(lo.MergeMaps(expectedSolidState, map[string]bool{
		"block1": true,
		"block2": true,
	}))

	tf.AssertInvalid(lo.MergeMaps(expectedInvalidState, map[string]bool{
		"block1": false,
		"block2": false,
	}))
	_, wasAttached, err = tf.Instance.Attach(tf.Block("block3"))
	require.NoError(t, err, "should not return an error")
	require.True(t, wasAttached, "should have been attached")
	workers.WaitChildren()

	tf.AssertSolid(lo.MergeMaps(expectedSolidState, map[string]bool{
		"block3": false,
	}))

	tf.AssertInvalid(lo.MergeMaps(expectedInvalidState, map[string]bool{
		"block3": true,
	}))
}

// This test prepares blocks across different slots and tries to attach them in reverse order to a pruned BlockDAG.
// At the end of the test only blocks from non-pruned slots should be attached and marked as invalid.
func TestBlockDAG_AttachInvalid(t *testing.T) {
	const slotCount = 100

	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	// create a helper function that creates the blocks
	createNewBlock := func(idx slot.Index, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 1 {
			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(tf.Instance.(*BlockDAG).SlotTimeProvider().GenesisTime()),
			), alias
		}
		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(tf.Instance.(*BlockDAG).SlotTimeProvider().StartTime(idx)),
		), alias
	}

	// Prune BlockDAG.
	tf.Instance.(*BlockDAG).evictionState.EvictUntil(slotCount / 2)
	workers.WaitChildren()

	require.EqualValues(t, slotCount/2, tf.Instance.(*BlockDAG).evictionState.LastEvictedSlot(), "maxDroppedSlot should be slotCount/2")

	blocks := make([]*models.Block, slotCount)
	expectedMissing := make(map[string]bool, slotCount)
	expectedInvalid := make(map[string]bool, slotCount)
	expectedSolid := make(map[string]bool, slotCount)

	// Prepare blocks and their expected states.
	for i := 0; i < slotCount; i++ {
		var alias string
		blocks[i], alias = createNewBlock(slot.Index(i+1), "")
		if i > 50 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = true
			expectedSolid[alias] = false
		} else if i == 50 {
			expectedMissing[alias] = true
			expectedInvalid[alias] = true
			expectedSolid[alias] = false
		}
	}

	// Issue the first bunch of blocks. Those should be marked as invalid when the block with pruned parents is attached.
	{
		for i := len(blocks) - 11; i >= 0; i-- {
			_, wasAttached, err := tf.Instance.Attach(blocks[i])
			if blocks[i].ID().Index()-1 > tf.Instance.(*BlockDAG).evictionState.LastEvictedSlot() {
				require.True(t, wasAttached, "block should be attached")
				require.NoError(t, err, "should not be able to attach a block after shutdown")
				continue
			}

			require.False(t, wasAttached, "block should not be attached")
			require.Error(t, err, "should not be able to attach a block to a pruned slot")
		}
		workers.WaitChildren()

		tf.AssertSolidCount(0, "should not have any solid blocks")
		tf.AssertInvalidCount(slotCount/2-10, "should have invalid blocks")
	}

	// Issue the second bunch of blocks. Those should be marked as invalid when the block attaches to a previously invalid block.
	{
		for i := len(blocks) - 1; i >= len(blocks)-10; i-- {
			_, wasAttached, err := tf.Instance.Attach(blocks[i])
			require.True(t, wasAttached, "block should be attached")
			require.NoError(t, err, "should not be able to attach a block after shutdown")
		}
		workers.WaitChildren()

		tf.AssertSolidCount(0, "should not have any solid blocks")
		tf.AssertInvalidCount(slotCount/2, "should have invalid blocks")
		tf.AssertMissing(expectedMissing)
		tf.AssertInvalid(expectedInvalid)
		tf.AssertSolid(expectedSolid)
	}
}

// This test creates two chains of blocks from the genesis (1 block per slot in each chain). The first chain is solid, the second chain is not.
// When evicting the BlockDAG, the first chain should be evicted but not marked as invalid by the causal order component, while the other should be marked as invalid.
func TestBlockDAG_Prune(t *testing.T) {
	const slotCount = 100

	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	// create a helper function that creates the blocks
	createNewBlock := func(idx slot.Index, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)

		if idx == 1 {
			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(tf.Instance.(*BlockDAG).SlotTimeProvider().GenesisTime()),
			), alias
		}

		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(tf.Instance.(*BlockDAG).SlotTimeProvider().StartTime(idx)),
		), alias
	}

	expectedMissing := make(map[string]bool, slotCount)
	expectedInvalid := make(map[string]bool, slotCount)
	expectedSolid := make(map[string]bool, slotCount)

	// Attach solid blocks
	for i := slot.Index(1); i <= slotCount; i++ {
		block, alias := createNewBlock(i, "")

		_, wasAttached, err := tf.Instance.Attach(block)

		if i >= slotCount/4 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = false
			expectedSolid[alias] = true
		}

		require.True(t, wasAttached, "block should be attached")
		require.NoError(t, err, "should not be able to attach a block after shutdown")
	}

	// Attach a blocks that are not solid (skip the first one in the chain)
	for i := slot.Index(1); i <= slotCount; i++ {
		blk, alias := createNewBlock(i, "-orphan")

		if i == 1 {
			continue
		}
		_, wasAttached, err := tf.Instance.Attach(blk)

		if i >= slotCount/2 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = true
			expectedSolid[alias] = false
		}

		require.True(t, wasAttached, "block should be attached")
		require.NoError(t, err, "should not be able to attach a block after shutdown")
	}
	workers.WaitChildren()

	tf.AssertSolidCount(slotCount, "should have all solid blocks")

	validateState(tf, 0, slotCount)
	tf.Instance.(*BlockDAG).evictionState.EvictUntil(slotCount / 4)
	workers.WaitChildren()
	require.EqualValues(t, slotCount/4, tf.Instance.(*BlockDAG).evictionState.LastEvictedSlot(), "maxDroppedSlot should be slotCount/4")

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	tf.AssertInvalidCount(slotCount, "should have invalid blocks")

	tf.Instance.(*BlockDAG).evictionState.EvictUntil(slotCount / 10)
	workers.WaitChildren()
	require.EqualValues(t, slotCount/4, tf.Instance.(*BlockDAG).evictionState.LastEvictedSlot(), "maxDroppedSlot should be slotCount/4")

	tf.Instance.(*BlockDAG).evictionState.EvictUntil(slotCount / 2)
	workers.WaitChildren()
	require.EqualValues(t, slotCount/2, tf.Instance.(*BlockDAG).evictionState.LastEvictedSlot(), "maxDroppedSlot should be slotCount/2")

	validateState(tf, slotCount/2, slotCount)
}

func validateState(tf *blockdag.TestFramework, maxDroppedSlot, slotCount int) {
	for i := 1; i <= maxDroppedSlot; i++ {
		blkID := tf.Block(fmt.Sprintf("blk-%d", i)).ID()

		_, exists := tf.Instance.Block(blkID)
		require.False(tf.Test, exists, "block %s should not be in the BlockDAG", blkID)

		require.Nil(tf.Test, tf.Instance.(*BlockDAG).memStorage.Get(blkID.Index()), "slot %s should not be in the memStorage", blkID.Index())
	}

	for i := maxDroppedSlot + 1; i <= slotCount; i++ {
		blkID := tf.Block(fmt.Sprintf("blk-%d", i)).ID()

		_, exists := tf.Instance.Block(blkID)
		require.True(tf.Test, exists, "block %s should be in the BlockDAG", blkID)

		require.NotNil(tf.Test, tf.Instance.(*BlockDAG).memStorage.Get(blkID.Index()), "slot %s should be in the memStorage", blkID.Index())
	}
}

func TestBlockDAG_MissingBlocks(t *testing.T) {
	const (
		blockCount    = 10000
		blockDAGWidth = 500
		storeDelay    = 0 * time.Millisecond
	)

	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	// map to keep track of the tips
	tips := randommap.New[models.BlockID, models.BlockID]()
	tips.Set(models.EmptyBlockID, models.EmptyBlockID)

	// create a helper function that creates the blocks
	createNewBlock := func(idx int) *models.Block {
		// issue the payload
		strongParents := make([]string, 0)
		for _, selectedTip := range tips.RandomUniqueEntries(2) {
			if selectedTip == models.EmptyBlockID {
				strongParents = append(strongParents, "Genesis")
				continue
			}
			strongParents = append(strongParents, selectedTip.Alias())
		}
		blk := tf.CreateBlock(fmt.Sprintf("msg-%d", idx), models.WithStrongParents(tf.BlockIDs(strongParents...)))
		// remove a tip if the width of the BlockDAG is reached
		if tips.Size() >= blockDAGWidth {
			tips.Delete(blk.ParentsByType(models.StrongParentType).First())
		}

		// add current block as a tip
		tips.Set(blk.ID(), blk.ID())

		// return the constructed block
		return blk
	}

	// generate the blocks we want to solidify
	blocks := make(map[models.BlockID]*models.Block, blockCount)
	for i := 0; i < blockCount; i++ {
		blk := createNewBlock(i)
		blocks[blk.ID()] = blk
	}

	tf.Instance.Events().BlockMissing.Hook(func(metadata *blockdag.Block) {
		time.Sleep(storeDelay)

		_, _, err := tf.Instance.Attach(blocks[metadata.ID()])
		require.NoError(t, err, "should be able to attach a block")
	}, event.WithWorkerPool(workers.CreatePool("BlockMissing", 2)))

	// issue tips to start solidification
	tips.ForEach(func(key models.BlockID, _ models.BlockID) bool {
		_, _, err := tf.Instance.Attach(blocks[key])
		require.NoError(t, err, "should be able to attach a block")
		return true
	})

	// wait until all blocks are solidified
	workers.WaitChildren()

	tf.AssertStoredCount(blockCount, "should have all blocks")
	tf.AssertInvalidCount(0, "should have no invalid blocks")
	tf.AssertSolidCount(blockCount, "should have all solid blocks")
	tf.AssertMissingCount(0, "should have no missing blocks")
}

func TestBlockDAG_MonotonicCommitments(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	tf.CreateBlock("block1", models.WithCommitment(commitment.New(0, commitment.ID{}, types.Identifier{}, 0)))
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")), models.WithCommitment(commitment.New(1, commitment.ID{}, types.Identifier{}, 0)))
	tf.CreateBlock("block3", models.WithStrongParents(tf.BlockIDs("block1", "block2")), models.WithCommitment(commitment.New(3, commitment.ID{}, types.Identifier{}, 0)))
	tf.CreateBlock("block4", models.WithStrongParents(tf.BlockIDs("block3", "block2")), models.WithCommitment(commitment.New(5, commitment.ID{}, types.Identifier{}, 0)))
	tf.CreateBlock("block5", models.WithStrongParents(tf.BlockIDs("block3", "block4")), models.WithCommitment(commitment.New(3, commitment.ID{}, types.Identifier{}, 0)))

	// issue block2
	{
		tf.IssueBlocks("block1", "block2", "block3", "block4", "block5")
		workers.WaitChildren()
		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
			"block3": true,
			"block4": true,
			"block5": false,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
			"block5": true,
		})
	}
}
