package blockdag

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
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
		workers.Wait()
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

		tf.AssertBlock("block1", func(block *Block) {
			require.True(t, block.ModelsBlock.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block1
	{
		tf.IssueBlocks("block1")
		workers.Wait()

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

		tf.AssertBlock("block1", func(block *Block) {
			require.False(t, block.ModelsBlock.IssuingTime().IsZero(), "block should be attached")
		})
	}

	// issue block4
	{
		tf.IssueBlocks("block4")
		workers.Wait()

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

		tf.AssertBlock("block3", func(block *Block) {
			require.True(t, block.ModelsBlock.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block5
	{
		tf.IssueBlocks("block5")
		workers.Wait()

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

		tf.AssertBlock("block3", func(block *Block) {
			require.True(t, block.ModelsBlock.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block3
	{
		tf.IssueBlocks("block3")
		workers.Wait()

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

		tf.AssertBlock("block3", func(block *Block) {
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
	workers.Wait()

	block1, _ := tf.Instance.Block(tf.Block("block1").ID())
	block2, _ := tf.Instance.Block(tf.Block("block2").ID())
	block4, _ := tf.Instance.Block(tf.Block("block4").ID())

	tf.Instance.SetOrphaned(block1, true)
	workers.Wait()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block1"))

	tf.Instance.SetOrphaned(block2, true)
	workers.Wait()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block1", "block2"))

	tf.Instance.SetOrphaned(block4, true)
	workers.Wait()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block1", "block2", "block4"))

	tf.Instance.SetOrphaned(block1, false)
	workers.Wait()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block2", "block4"))

	tf.Instance.SetOrphaned(block2, false)
	workers.Wait()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block4"))

	tf.IssueBlocks("block6")
	workers.Wait()
	tf.AssertOrphanedBlocks(tf.BlockIDs("block4"))

	tf.Instance.SetOrphaned(block4, false)
	workers.Wait()
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

	workers.Wait()

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

	workers.Wait()

	require.NoError(t, err, "should not return an error")
}

func TestBlockDAG_Attach_InvalidTimestamp(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	now := time.Now()
	tf.CreateBlock("block1", models.WithIssuingTime(now.Add(-5*time.Second)))
	tf.CreateBlock("block2", models.WithIssuingTime(now.Add(5*time.Second)))
	tf.CreateBlock("block3", models.WithStrongParents(tf.BlockIDs("block1", "block2")), models.WithIssuingTime(now))

	_, wasAttached, err := tf.Instance.Attach(tf.Block("block1"))
	require.NoError(t, err, "should not return an error")
	require.True(t, wasAttached, "should have been attached")

	_, wasAttached, err = tf.Instance.Attach(tf.Block("block2"))
	require.NoError(t, err, "should not return an error")
	require.True(t, wasAttached, "should have been attached")

	workers.Wait()

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
	workers.Wait()

	tf.AssertSolid(lo.MergeMaps(expectedSolidState, map[string]bool{
		"block3": false,
	}))

	tf.AssertInvalid(lo.MergeMaps(expectedInvalidState, map[string]bool{
		"block3": true,
	}))
}

// This test prepares blocks across different epochs and tries to attach them in reverse order to a pruned BlockDAG.
// At the end of the test only blocks from non-pruned epochs should be attached and marked as invalid.
func TestBlockDAG_AttachInvalid(t *testing.T) {
	const epochCount = 100

	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 0 {
			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			), alias
		}
		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)),
		), alias
	}

	// Prune BlockDAG.
	tf.Instance.EvictionState.EvictUntil(epochCount / 2)
	workers.Wait()

	require.EqualValues(t, epochCount/2, tf.Instance.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch should be epochCount/2")

	blocks := make([]*models.Block, epochCount)
	expectedMissing := make(map[string]bool, epochCount)
	expectedInvalid := make(map[string]bool, epochCount)
	expectedSolid := make(map[string]bool, epochCount)

	// Prepare blocks and their expected states.
	for i := 0; i < epochCount; i++ {
		var alias string
		blocks[i], alias = createNewBlock(i, "")
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
			if blocks[i].ID().Index()-1 > tf.Instance.EvictionState.LastEvictedEpoch() {
				require.True(t, wasAttached, "block should be attached")
				require.NoError(t, err, "should not be able to attach a block after shutdown")
				continue
			}

			require.False(t, wasAttached, "block should not be attached")
			require.Error(t, err, "should not be able to attach a block to a pruned epoch")
		}
		workers.Wait()

		tf.AssertSolidCount(0, "should not have any solid blocks")
		tf.AssertInvalidCount(epochCount/2-10, "should have invalid blocks")
	}

	// Issue the second bunch of blocks. Those should be marked as invalid when the block attaches to a previously invalid block.
	{
		for i := len(blocks) - 1; i >= len(blocks)-10; i-- {
			_, wasAttached, err := tf.Instance.Attach(blocks[i])
			require.True(t, wasAttached, "block should be attached")
			require.NoError(t, err, "should not be able to attach a block after shutdown")
		}
		workers.Wait()

		tf.AssertSolidCount(0, "should not have any solid blocks")
		tf.AssertInvalidCount(epochCount/2, "should have invalid blocks")
		tf.AssertMissing(expectedMissing)
		tf.AssertInvalid(expectedInvalid)
		tf.AssertSolid(expectedSolid)
	}
}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When evicting the BlockDAG, the first chain should be evicted but not marked as invalid by the causal order component, while the other should be marked as invalid.
func TestBlockDAG_Prune(t *testing.T) {
	const epochCount = 100

	workers := workerpool.NewGroup(t.Name())
	tf := NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)

		if idx == 1 {
			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			), alias
		}

		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx-1)*epoch.Duration, 0)),
		), alias
	}

	expectedMissing := make(map[string]bool, epochCount)
	expectedInvalid := make(map[string]bool, epochCount)
	expectedSolid := make(map[string]bool, epochCount)

	// Attach solid blocks
	for i := 1; i <= epochCount; i++ {
		block, alias := createNewBlock(i, "")

		_, wasAttached, err := tf.Instance.Attach(block)

		if i >= epochCount/4 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = false
			expectedSolid[alias] = true
		}

		require.True(t, wasAttached, "block should be attached")
		require.NoError(t, err, "should not be able to attach a block after shutdown")
	}

	// Attach a blocks that are not solid (skip the first one in the chain)
	for i := 1; i <= epochCount; i++ {
		blk, alias := createNewBlock(i, "-orphan")

		if i == 1 {
			continue
		}
		_, wasAttached, err := tf.Instance.Attach(blk)

		if i >= epochCount/2 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = true
			expectedSolid[alias] = false
		}

		require.True(t, wasAttached, "block should be attached")
		require.NoError(t, err, "should not be able to attach a block after shutdown")
	}
	workers.Wait()

	tf.AssertSolidCount(epochCount, "should have all solid blocks")

	validateState(tf, 0, epochCount)
	tf.Instance.EvictionState.EvictUntil(epochCount / 4)
	workers.Wait()
	require.EqualValues(t, epochCount/4, tf.Instance.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch should be epochCount/4")

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	tf.AssertInvalidCount(epochCount, "should have invalid blocks")

	tf.Instance.EvictionState.EvictUntil(epochCount / 10)
	workers.Wait()
	require.EqualValues(t, epochCount/4, tf.Instance.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch should be epochCount/4")

	tf.Instance.EvictionState.EvictUntil(epochCount / 2)
	workers.Wait()
	require.EqualValues(t, epochCount/2, tf.Instance.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch should be epochCount/2")

	validateState(tf, epochCount/2, epochCount)
}

func validateState(tf *TestFramework, maxDroppedEpoch, epochCount int) {
	for i := 1; i <= maxDroppedEpoch; i++ {
		blkID := tf.Block(fmt.Sprintf("blk-%d", i)).ID()

		_, exists := tf.Instance.Block(blkID)
		require.False(tf.test, exists, "block %s should not be in the BlockDAG", blkID)

		require.Nil(tf.test, tf.Instance.memStorage.Get(blkID.Index()), "epoch %s should not be in the memStorage", blkID.Index())
	}

	for i := maxDroppedEpoch + 1; i <= epochCount; i++ {
		blkID := tf.Block(fmt.Sprintf("blk-%d", i)).ID()

		_, exists := tf.Instance.Block(blkID)
		require.True(tf.test, exists, "block %s should be in the BlockDAG", blkID)

		require.NotNil(tf.test, tf.Instance.memStorage.Get(blkID.Index()), "epoch %s should be in the memStorage", blkID.Index())
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

	event.AttachWithWorkerPool(tf.Instance.Events.BlockMissing, func(metadata *Block) {
		time.Sleep(storeDelay)

		_, _, err := tf.Instance.Attach(blocks[metadata.ID()])
		require.NoError(t, err, "should be able to attach a block")
	}, workers.CreatePool("BlockMissing", 2))

	// issue tips to start solidification
	tips.ForEach(func(key models.BlockID, _ models.BlockID) bool {
		_, _, err := tf.Instance.Attach(blocks[key])
		require.NoError(t, err, "should be able to attach a block")
		return true
	})

	// wait until all blocks are solidified
	workers.Wait()

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
		workers.Wait()
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
