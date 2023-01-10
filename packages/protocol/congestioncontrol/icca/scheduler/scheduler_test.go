package scheduler

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Scheduler_test /////////////////////////////////////////////////////////////////////////////////////////////

func TestScheduler_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	tf := NewTestFramework(t)
	tf.Scheduler.Start()

	time.Sleep(100 * time.Millisecond)
	tf.Scheduler.Shutdown()
}

func TestScheduler_AddBlock(t *testing.T) {
	tf := NewTestFramework(t)

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	blk := virtualvoting.NewBlock(booker.NewBlock(blockdag.NewBlock(models.NewBlock(models.WithStrongParents(tf.BlockIDs("Genesis"))), blockdag.WithSolid(true), blockdag.WithOrphaned(true)), booker.WithBooked(true), booker.WithStructureDetails(markers.NewStructureDetails())))
	assert.NoError(t, blk.DetermineID())

	tf.Scheduler.AddBlock(blk)

	schedulerBlock, exists := tf.Scheduler.Block(blk.ID())

	assert.True(t, exists, "scheduler block should exist")
	assert.True(t, schedulerBlock.IsDropped(), "block should be dropped")
	tf.AssertBlocksDropped(1)
	tf.AssertBlocksSkipped(0)

}

func TestScheduler_Submit(t *testing.T) {
	tf := NewTestFramework(t)

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	blk := tf.CreateSchedulerBlock(models.WithIssuer(selfNode.PublicKey()))
	assert.NoError(t, tf.Scheduler.Submit(blk))
	time.Sleep(100 * time.Millisecond)
	// unsubmit to allow the scheduler to shutdown
	tf.Scheduler.Unsubmit(blk)
}

func TestScheduler_updateActiveNodeList(t *testing.T) {
	tf := NewTestFramework(t)

	tf.Scheduler.updateActiveIssuersList(map[identity.ID]int64{})
	assert.Equal(t, 0, tf.Scheduler.buffer.NumActiveIssuers())
	for _, alias := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		tf.CreateIssuer(alias, 0)
	}
	tf.UpdateIssuers(map[string]int64{
		"A": 30,
		"B": 15,
		"C": 25,
		"D": 20,
		"E": 10,
		"G": 0,
	})
	tf.Scheduler.updateActiveIssuersList(tf.ManaMap())

	assert.Equal(t, 5, tf.Scheduler.buffer.NumActiveIssuers())
	issuerIDs := tf.Scheduler.buffer.IssuerIDs()
	assert.Contains(t, issuerIDs, tf.Issuer("A").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("B").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("C").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("D").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("E").ID())
	assert.NotContains(t, issuerIDs, tf.Issuer("F").ID())
	assert.NotContains(t, issuerIDs, tf.Issuer("G").ID())
	tf.UpdateIssuers(map[string]int64{
		"A": 30,
		"B": 15,
		"C": 25,
		"E": 0,
		"F": 1,
		"G": 5,
	})
	tf.Scheduler.updateActiveIssuersList(tf.ManaMap())

	assert.Equal(t, 5, tf.Scheduler.buffer.NumActiveIssuers())

	issuerIDs = tf.Scheduler.buffer.IssuerIDs()
	assert.Contains(t, issuerIDs, tf.Issuer("A").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("B").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("C").ID())
	assert.NotContains(t, issuerIDs, tf.Issuer("D").ID())
	assert.NotContains(t, issuerIDs, tf.Issuer("E").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("F").ID())
	assert.Contains(t, issuerIDs, tf.Issuer("G").ID())

	tf.Scheduler.updateActiveIssuersList(map[identity.ID]int64{})
	assert.Equal(t, 0, tf.Scheduler.buffer.NumActiveIssuers())
}

func TestScheduler_Dropped(t *testing.T) {
	tf := NewTestFramework(t, WithSchedulerOptions(WithMaxBufferSize(numBlocks/2)))

	tf.CreateIssuer("nomana", 0)

	droppedBlockIDChan := make(chan models.BlockID, numBlocks)
	tf.Scheduler.Events.BlockDropped.Hook(event.NewClosure(func(block *Block) {
		droppedBlockIDChan <- block.ID()
	}))

	for i := 0; i < numBlocks; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		tf.CreateBlock(alias, models.WithIssuer(tf.Issuer("nomana").PublicKey()), models.WithStrongParents(models.NewBlockIDs(tf.Block("Genesis").ID())))
		tf.IssueBlocks(alias).WaitUntilAllTasksProcessed()
	}
	droppedCounter := 0
	assert.Eventually(t, func() bool {
		select {
		case droppedBlockID := <-droppedBlockIDChan:
			expectedBlock := tf.Block(fmt.Sprintf("blk-%d", droppedCounter))
			droppedCounter++
			return assert.Equal(t, expectedBlock.ID(), droppedBlockID)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
	tf.AssertBlocksDropped(numBlocks / 2)
}

func TestScheduler_Schedule(t *testing.T) {
	tf := NewTestFramework(t)

	blockScheduled := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		blockScheduled <- block.ID()
	}))
	tf.CreateIssuer("peer", 10)

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()
	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	assert.NoError(t, tf.Scheduler.Submit(blk))
	tf.Scheduler.Ready(blk)

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			return assert.Equal(t, blk.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_HandleOrphanedBlock_Ready(t *testing.T) {
	tf := NewTestFramework(t)

	blockDropped := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockDropped.Hook(event.NewClosure(func(block *Block) {
		blockDropped <- block.ID()
	}))

	tf.CreateIssuer("peer", 10)
	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	assert.NoError(t, tf.Scheduler.Submit(blk))
	tf.Scheduler.Ready(blk)
	tf.Tangle.SetOrphaned(blk.Block.Block.Block, true)

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockDropped:
			return assert.Equal(t, blk.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
	tf.AssertBlocksDropped(1)
	tf.AssertBlocksScheduled(0)
}

func TestScheduler_HandleOrphanedBlock_Scheduled(t *testing.T) {
	tf := NewTestFramework(t)

	blockScheduled := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		blockScheduled <- block.ID()
	}))
	tf.CreateIssuer("peer", 10)

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	assert.NoError(t, tf.Scheduler.Submit(blk))
	tf.Scheduler.Ready(blk)

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			return assert.Equal(t, blk.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	tf.Tangle.SetOrphaned(blk.Block.Block.Block, true)

	tf.AssertBlocksDropped(0)
}

func TestScheduler_HandleOrphanedBlock_Unready(t *testing.T) {
	tf := NewTestFramework(t)

	blockDropped := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockDropped.Hook(event.NewClosure(func(block *Block) {
		blockDropped <- block.ID()
	}))

	tf.CreateIssuer("peer", 10)
	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	assert.NoError(t, tf.Scheduler.Submit(blk))
	tf.Tangle.SetOrphaned(blk.Block.Block.Block, true)

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockDropped:
			return assert.Equal(t, blk.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
	tf.AssertBlocksScheduled(0)
}

func TestScheduler_SkipConfirmed(t *testing.T) {
	tf := NewTestFramework(t, WithSchedulerOptions(WithAcceptedBlockScheduleThreshold(time.Minute)))
	tf.CreateIssuer("peer", 10)

	blockScheduled := make(chan models.BlockID, 1)
	blockSkipped := make(chan models.BlockID, 1)

	tf.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		blockScheduled <- block.ID()
	}))
	tf.Scheduler.Events.BlockSkipped.Hook(event.NewClosure(func(block *Block) {
		blockSkipped <- block.ID()
	}))

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	// create a new block from a different node and mark it as ready and confirmed, but younger than 1 minute
	blkReadyConfirmedNew := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkReadyConfirmedNew.ID(): types.Void,
	})

	assert.NoError(t, tf.Scheduler.Submit(blkReadyConfirmedNew))
	tf.Scheduler.Ready(blkReadyConfirmedNew)

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			return assert.Equal(t, blkReadyConfirmedNew.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as unready and confirmed, but younger than 1 minute
	blkUnreadyConfirmedNew := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))

	assert.NoError(t, tf.Scheduler.Submit(blkUnreadyConfirmedNew))

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkUnreadyConfirmedNew.ID(): types.Void,
	})

	tf.mockAcceptance.BlockAcceptedEvent.Trigger(blockgadget.NewBlock(blkUnreadyConfirmedNew.Block, blockgadget.WithAccepted(true)))

	// make sure that the block was not unsubmitted
	assert.Equal(t, tf.Scheduler.buffer.IssuerQueue(tf.Issuer("peer").ID()).IDs()[0], blkUnreadyConfirmedNew.ID())
	tf.Scheduler.Ready(blkUnreadyConfirmedNew)

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			return assert.Equal(t, blkUnreadyConfirmedNew.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as ready and confirmed, but older than 1 minute
	blkReadyConfirmedOld := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(-2*time.Minute)))

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkReadyConfirmedOld.ID(): types.Void,
	})

	assert.NoError(t, tf.Scheduler.Submit(blkReadyConfirmedOld))
	tf.Scheduler.Ready(blkReadyConfirmedOld)

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockSkipped:
			return assert.Equal(t, blkReadyConfirmedOld.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as unready and confirmed, but older than 1 minute
	blkUnreadyConfirmedOld := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(-2*time.Minute)))
	assert.NoError(t, tf.Scheduler.Submit(blkUnreadyConfirmedOld))
	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkUnreadyConfirmedOld.ID(): types.Void,
	})

	tf.mockAcceptance.BlockAcceptedEvent.Trigger(blockgadget.NewBlock(blkUnreadyConfirmedOld.Block, blockgadget.WithAccepted(true)))

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockSkipped:
			return assert.Equal(t, blkUnreadyConfirmedOld.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_Time(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateIssuer("peer", 10)

	blockScheduled := make(chan *Block, 1)
	tf.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		blockScheduled <- block
	}))

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	futureBlock := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(1*time.Second)))
	assert.NoError(t, tf.Scheduler.Submit(futureBlock))

	nowBlock := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	assert.NoError(t, tf.Scheduler.Submit(nowBlock))

	tf.Scheduler.Ready(futureBlock)
	tf.Scheduler.Ready(nowBlock)

	done := make(chan struct{})
	scheduledIDs := models.NewBlockIDs()
	go func() {
		defer close(done)
		timer := time.NewTimer(time.Until(futureBlock.IssuingTime()) + 100*time.Millisecond)
		for {
			select {
			case <-timer.C:
				return
			case block := <-blockScheduled:
				assert.Truef(t, time.Now().After(block.IssuingTime()), "scheduled too early: %s", time.Until(block.IssuingTime()))
				scheduledIDs.Add(block.ID())
			}
		}
	}()

	<-done
	assert.Equal(t, models.NewBlockIDs(nowBlock.ID(), futureBlock.ID()), scheduledIDs)
}

func TestScheduler_Issue(t *testing.T) {
	debug.SetEnabled(true)
	tf := NewTestFramework(t)
	tf.CreateIssuer("peer", 10)

	tf.Scheduler.Events.Error.Hook(event.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	// setup tangle up till the Scheduler
	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	const numBlocks = 5
	blockScheduled := make(chan models.BlockID, numBlocks)
	tf.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		blockScheduled <- block.ID()
	}))

	ids := models.NewBlockIDs()
	for i := 0; i < numBlocks; i++ {
		block := tf.CreateBlock(fmt.Sprintf("blk-%d", i), models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithStrongParents(models.NewBlockIDs(tf.Block("Genesis").ID())))
		ids.Add(block.ID())
		_, _, err := tf.Tangle.Attach(block)
		assert.NoError(t, err)
	}

	scheduledIDs := models.NewBlockIDs()
	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			scheduledIDs.Add(id)
			return len(scheduledIDs) == len(ids)
		default:
			return false
		}
	}, 10*time.Second, 10*time.Millisecond)
	assert.Equal(t, ids, scheduledIDs)

	for i := 0; i < numBlocks; i++ {
		block, _ := tf.Scheduler.Block(tf.Block(fmt.Sprintf("blk-%d", i)).ID())
		lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
			block.ID(): types.Void,
		})
		tf.mockAcceptance.BlockAcceptedEvent.Trigger(blockgadget.NewBlock(block.Block, blockgadget.WithAccepted(true)))
	}

	tf.AssertBlocksSkipped(0)
	tf.AssertBlocksScheduled(numBlocks)
}

func TestSchedulerFlow(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateIssuer("peer", 10)
	tf.CreateIssuer("self", 10)

	tf.Scheduler.Events.Error.Hook(event.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	// testing desired scheduled order: A - B - D - E - C
	tf.CreateBlock("A", models.WithIssuer(tf.Issuer("self").PublicKey()), models.WithStrongParents(tf.BlockIDs("Genesis")))
	tf.CreateBlock("B", models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(1*time.Second)), models.WithStrongParents(tf.BlockIDs("Genesis")))

	// set C to have a timestamp in the future
	tf.CreateBlock("C", models.WithIssuer(tf.Issuer("self").PublicKey()), models.WithIssuingTime(time.Now().Add(5*time.Second)), models.WithStrongParents(tf.BlockIDs("A", "B")))

	tf.CreateBlock("D", models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(1*time.Second)), models.WithStrongParents(tf.BlockIDs("A", "B")))

	tf.CreateBlock("E", models.WithIssuer(tf.Issuer("self").PublicKey()), models.WithIssuingTime(time.Now().Add(3*time.Second)), models.WithStrongParents(tf.BlockIDs("A", "B")))

	blockScheduled := make(chan models.BlockID, 5)
	tf.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		blockScheduled <- block.ID()
	}))

	tf.IssueBlocks("A", "B", "C", "D", "E").WaitUntilAllTasksProcessed()

	var scheduledIDs []models.BlockID
	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			scheduledIDs = append(scheduledIDs, id)
			return len(scheduledIDs) == 5
		default:
			return false
		}
	}, 10*time.Second, 100*time.Millisecond)

	assert.Equal(t, scheduledIDs, []models.BlockID{tf.Block("A").ID(), tf.Block("B").ID(), tf.Block("D").ID(), tf.Block("E").ID(), tf.Block("C").ID()})
}

func TestSchedulerParallelSubmit(t *testing.T) {
	debug.SetEnabled(true)
	const totalBlkCount = 200

	tf := NewTestFramework(t)

	tf.Scheduler.Events.Error.Hook(event.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	tf.CreateIssuer("self", 10)
	tf.CreateIssuer("peer", 10)

	tf.Scheduler.Start()
	defer tf.Scheduler.Shutdown()

	// generate the blocks we want to solidify
	blockAliases := make([]string, 0, totalBlkCount)

	for i := 0; i < totalBlkCount/2; i++ {
		alias := fmt.Sprintf("blk-%d", i)

		parentOption := models.WithStrongParents(tf.BlockIDs("Genesis"))
		if i > 1 {
			parentOption = models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk-%d", i-1), fmt.Sprintf("blk-%d", i-2)))
		}

		tf.CreateBlock(alias, models.WithIssuer(tf.Issuer("self").PublicKey()), parentOption)
		blockAliases = append(blockAliases, alias)
	}

	for i := totalBlkCount / 2; i < totalBlkCount; i++ {
		alias := fmt.Sprintf("blk-%d", i)

		tf.CreateBlock(alias, models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk-%d", i-1), fmt.Sprintf("blk-%d", i-2))))
		blockAliases = append(blockAliases, alias)
	}

	// issue tips to start solidification
	tf.IssueBlocks(blockAliases...).WaitUntilAllTasksProcessed()

	// wait for all blocks to have a formed opinion
	assert.Eventually(t, func() bool { return atomic.LoadUint32(&(tf.scheduledBlocksCount)) == totalBlkCount }, 5*time.Minute, 100*time.Millisecond)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
