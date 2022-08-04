package tangle

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/randommap"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// This test checks if the internal metadata is correct i.e. that children are assigned correctly and that all the flags are correct.
func TestTangle_AttachBlock(t *testing.T) {
	tf := NewTestFramework(t)
	defer tf.Shutdown()
	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))

	{
		tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
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
			assert.True(t, block.Block.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	{
		tf.IssueBlocks("block1").WaitUntilAllTasksProcessed()

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
			assert.False(t, block.Block.IssuingTime().IsZero(), "block should be attached")
		})
	}
	// TODO: issue more blocks to check that children are updated correctly
}

func TestTangle_Shutdown(t *testing.T) {
	testFramework := NewTestFramework(t)
	testFramework.Tangle.Shutdown()

	newBlock := testFramework.CreateBlock("block")

	_, _, err := testFramework.Tangle.Attach(newBlock)
	assert.Error(t, err, "should not be able to attach a block after shutdown")

	_, exists := testFramework.Tangle.Block(newBlock.ID())
	assert.False(t, exists, "block should not be in the tangle")

	wasUpdated := testFramework.Tangle.SetInvalid(&Block{})
	assert.False(t, wasUpdated, "block should not be updated")
}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When pruning the tangle, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func TestTangle_AttachInvalid(t *testing.T) {
	t.Skip()
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) *models.Block {
		if idx == 0 {
			return tf.CreateBlock(
				fmt.Sprintf("blk%s-%d", prefix, idx),
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			)
		}
		return tf.CreateBlock(
			fmt.Sprintf("blk%s-%d", prefix, idx),
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)),
		)
	}

	assert.EqualValues(t, 0, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be 0")
	tf.Tangle.Prune(epochCount / 2)
	assert.EqualValues(t, epochCount/2, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/2")
	event.Loop.WaitUntilAllTasksProcessed()

	blocks := make([]*models.Block, epochCount)
	// Attach solid blocks
	for i := 0; i < epochCount; i++ {
		// TODO: validate each block separately
		blocks[i] = createNewBlock(i, "")
	}

	// Attach a blocks that are not solid (skip the first one in the chain
	for i := len(blocks) - 1; i >= 0; i-- {
		fmt.Println("Attaching block", blocks[i].ID())
		_, wasAttached, err := tf.Tangle.Attach(blocks[i])
		if blocks[i].ID().Index() > tf.Tangle.maxDroppedEpoch {
			assert.True(t, wasAttached, "block should be attached")
			assert.NoError(t, err, "should not be able to attach a block after shutdown")
			continue
		}
		assert.False(t, wasAttached, "block should not be attached")
		assert.Error(t, err, "should not be able to attach a block to a pruned epoch")
	}
	event.Loop.WaitUntilAllTasksProcessed()

	tf.AssertSolidCount(0, "should not have any solid blocks")
	tf.AssertInvalidCount(epochCount/2, "should have invalid blocks")
}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When pruning the tangle, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func TestTangle_Prune(t *testing.T) {
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) *models.Block {
		if idx == 0 {
			return tf.CreateBlock(
				fmt.Sprintf("blk%s-%d", prefix, idx),
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			)
		}
		return tf.CreateBlock(
			fmt.Sprintf("blk%s-%d", prefix, idx),
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)),
		)
	}

	assert.EqualValues(t, -1, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be 0")

	// Attach solid blocks
	for i := 0; i < epochCount; i++ {
		_, wasAttached, err := tf.Tangle.Attach(createNewBlock(i, ""))
		assert.True(t, wasAttached, "block should be attached")
		assert.NoError(t, err, "should not be able to attach a block after shutdown")
	}

	// Attach a blocks that are not solid (skip the first one in the chain
	for i := 0; i < epochCount; i++ {
		// TODO: check the state of each individual metadata
		blk := createNewBlock(i, "-orphan")
		if i == 0 {
			continue
		}
		_, wasAttached, err := tf.Tangle.Attach(blk)
		assert.True(t, wasAttached, "block should be attached")
		assert.NoError(t, err, "should not be able to attach a block after shutdown")
	}
	tf.WaitUntilAllTasksProcessed()

	tf.AssertSolidCount(epochCount, "should have all solid blocks")

	_, exists := tf.Tangle.Block(tf.Block("blk-0").ID())
	assert.True(t, exists, "block should be in the tangle")

	tf.Tangle.Prune(epochCount / 4)
	tf.WaitUntilAllTasksProcessed()

	assert.EqualValues(t, epochCount/4, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/4")

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	tf.AssertInvalidCount(epochCount, "should have invalid blocks")

	tf.Tangle.Prune(epochCount / 10)
	assert.EqualValues(t, epochCount/4, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/4")

	tf.Tangle.Prune(epochCount / 2)
	assert.EqualValues(t, epochCount/2, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/2")

	_, exists = tf.Tangle.Block(tf.Block("blk-0").ID())
	assert.False(t, exists, "block should not be in the tangle")
}

func TestTangle_MissingBlocks(t *testing.T) {
	const (
		blockCount  = 10000
		tangleWidth = 500
		storeDelay  = 0 * time.Millisecond
	)

	tf := NewTestFramework(t)
	defer tf.Shutdown()

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
		// remove a tip if the width of the tangle is reached
		if tips.Size() >= tangleWidth {
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

	tf.Tangle.Events.BlockMissing.Attach(event.NewClosure(func(metadata *Block) {
		time.Sleep(storeDelay)

		_, _, err := tf.Tangle.Attach(blocks[metadata.ID()])
		assert.NoError(t, err, "should be able to attach a block")
	}))

	// issue tips to start solidification
	tips.ForEach(func(key models.BlockID, _ models.BlockID) {
		_, _, err := tf.Tangle.Attach(blocks[key])
		assert.NoError(t, err, "should be able to attach a block")
	})

	// wait until all blocks are solidified
	tf.WaitUntilAllTasksProcessed()

	tf.AssertStoredCount(blockCount, "should have all blocks")
	tf.AssertInvalidCount(0, "should have no invalid blocks")
	tf.AssertSolidCount(blockCount, "should have all solid blocks")
	tf.AssertMissingCount(0, "should have no missing blocks")
}
