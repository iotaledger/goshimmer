package scheduler

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Scheduler_test /////////////////////////////////////////////////////////////////////////////////////////////

func TestScheduler_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))
	tf.Scheduler.Start()
	// Multiple calls to start should not create problems
	tf.Scheduler.Start()
	tf.Scheduler.Start()

	time.Sleep(100 * time.Millisecond)
	tf.Scheduler.Shutdown()
	// Multiple calls to shutdown should not create problems
	tf.Scheduler.Shutdown()
	tf.Scheduler.Shutdown()
}

func TestScheduler_AddBlock(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))
	tf.Scheduler.Start()

	blk := booker.NewBlock(blockdag.NewBlock(models.NewBlock(models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis"))), blockdag.WithSolid(true), blockdag.WithOrphaned(true)), booker.WithBooked(true), booker.WithStructureDetails(markers.NewStructureDetails()))
	require.NoError(t, blk.DetermineID(tf.SlotTimeProvider()))

	tf.Scheduler.AddBlock(blk)

	schedulerBlock, exists := tf.Scheduler.Block(blk.ID())

	require.True(t, exists, "scheduler block should exist")
	require.True(t, schedulerBlock.IsDropped(), "block should be dropped")
	tf.AssertBlocksDropped(1)
	tf.AssertBlocksSkipped(0)
}

func TestScheduler_Submit(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))
	tf.Scheduler.Start()

	blk := tf.CreateSchedulerBlock(models.WithIssuer(selfNode.PublicKey()))
	require.NoError(t, tf.Scheduler.Submit(blk))
	time.Sleep(100 * time.Millisecond)
	// unsubmit to allow the scheduler to shutdown
	tf.Scheduler.Unsubmit(blk)
}

func TestScheduler_updateActiveNodeList(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.Scheduler.updateActiveIssuersList(map[identity.ID]int64{})
	require.Equal(t, 0, tf.Scheduler.buffer.NumActiveIssuers())
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

	require.Equal(t, 5, tf.Scheduler.buffer.NumActiveIssuers())
	issuerIDs := tf.Scheduler.buffer.IssuerIDs()
	require.Contains(t, issuerIDs, tf.Issuer("A").ID())
	require.Contains(t, issuerIDs, tf.Issuer("B").ID())
	require.Contains(t, issuerIDs, tf.Issuer("C").ID())
	require.Contains(t, issuerIDs, tf.Issuer("D").ID())
	require.Contains(t, issuerIDs, tf.Issuer("E").ID())
	require.NotContains(t, issuerIDs, tf.Issuer("F").ID())
	require.NotContains(t, issuerIDs, tf.Issuer("G").ID())
	tf.UpdateIssuers(map[string]int64{
		"A": 30,
		"B": 15,
		"C": 25,
		"E": 0,
		"F": 1,
		"G": 5,
	})
	tf.Scheduler.updateActiveIssuersList(tf.ManaMap())

	require.Equal(t, 5, tf.Scheduler.buffer.NumActiveIssuers())

	issuerIDs = tf.Scheduler.buffer.IssuerIDs()
	require.Contains(t, issuerIDs, tf.Issuer("A").ID())
	require.Contains(t, issuerIDs, tf.Issuer("B").ID())
	require.Contains(t, issuerIDs, tf.Issuer("C").ID())
	require.NotContains(t, issuerIDs, tf.Issuer("D").ID())
	require.NotContains(t, issuerIDs, tf.Issuer("E").ID())
	require.Contains(t, issuerIDs, tf.Issuer("F").ID())
	require.Contains(t, issuerIDs, tf.Issuer("G").ID())

	tf.Scheduler.updateActiveIssuersList(map[identity.ID]int64{})
	require.Equal(t, 0, tf.Scheduler.buffer.NumActiveIssuers())
}

func TestScheduler_Dropped(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"), WithMaxBufferSize(numBlocks/2))

	tf.CreateIssuer("nomana", 0)
	tf.CreateIssuer("other", 10) // Add a second issuer so that totalMana is not zero!

	type schedulerBlock struct {
		ID      models.BlockID
		Dropped bool
	}

	processedBlocksChan := make(chan schedulerBlock, numBlocks)
	tf.Scheduler.Events.BlockDropped.Hook(func(block *Block) {
		processedBlocksChan <- schedulerBlock{ID: block.ID(), Dropped: true}
		require.Equal(t, numBlocks/2, tf.Scheduler.buffer.Size()) // If a block is dropped it is because the buffer is full
	})

	tf.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		processedBlocksChan <- schedulerBlock{ID: block.ID(), Dropped: false}
	})

	tf.Scheduler.Start()

	for i := 0; i < numBlocks; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		tf.Tangle.BlockDAG.CreateBlock(alias, models.WithIssuer(tf.Issuer("nomana").PublicKey()), models.WithStrongParents(models.NewBlockIDs(tf.Tangle.BlockDAG.Block("Genesis").ID())))
		tf.Tangle.BlockDAG.IssueBlocks(alias)
	}
	blockCounter := 0
	require.Eventually(t, func() bool {
		expectedBlock := tf.Tangle.BlockDAG.Block(fmt.Sprintf("blk-%d", blockCounter))
		select {
		case droppedBlock := <-processedBlocksChan:
			blockCounter++
			require.Equal(t, expectedBlock.ID(), droppedBlock.ID) // Blocks should be processed in a FIFO order
			return droppedBlock.Dropped                           // We can stop as soon as the first block is dropped
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_Schedule(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.CreateIssuer("peer", 10)

	blockScheduled := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		blockScheduled <- block.ID()
	})

	tf.Scheduler.Start()

	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	require.NoError(t, tf.Scheduler.Submit(blk))
	tf.Scheduler.Ready(blk)

	require.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			require.Equal(t, blk.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_HandleOrphanedBlock_Ready(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.CreateIssuer("peer", 10)

	blockDropped := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockDropped.Hook(func(block *Block) {
		blockDropped <- block.ID()
	})

	tf.Scheduler.Start()

	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	require.NoError(t, tf.Scheduler.Submit(blk))
	tf.Scheduler.Ready(blk)
	tf.Tangle.BlockDAG.Instance.SetOrphaned(blk.Block.Block, true)

	require.Eventually(t, func() bool {
		select {
		case id := <-blockDropped:
			require.Equal(t, blk.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
	tf.AssertBlocksDropped(1)
	tf.AssertBlocksScheduled(0)
}

func TestScheduler_HandleOrphanedBlock_Scheduled(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.CreateIssuer("peer", 10)

	blockScheduled := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		blockScheduled <- block.ID()
	})

	tf.Scheduler.Start()

	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	require.NoError(t, tf.Scheduler.Submit(blk))
	tf.Scheduler.Ready(blk)

	require.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			require.Equal(t, blk.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	tf.Tangle.BlockDAG.Instance.SetOrphaned(blk.Block.Block, true)

	tf.AssertBlocksDropped(0)
}

func TestScheduler_HandleOrphanedBlock_Unready(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.CreateIssuer("peer", 10)

	blockDropped := make(chan models.BlockID, 1)
	tf.Scheduler.Events.BlockDropped.Hook(func(block *Block) {
		blockDropped <- block.ID()
	})

	tf.Scheduler.Start()

	// create a new block from a different node
	blk := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	require.NoError(t, tf.Scheduler.Submit(blk))
	tf.Tangle.BlockDAG.Instance.SetOrphaned(blk.Block.Block, true)

	require.Eventually(t, func() bool {
		select {
		case id := <-blockDropped:
			require.Equal(t, blk.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
	tf.AssertBlocksScheduled(0)
}

func TestScheduler_SkipConfirmed(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"), WithAcceptedBlockScheduleThreshold(time.Minute))

	tf.CreateIssuer("peer", 10)

	blockScheduled := make(chan models.BlockID, 1)
	blockSkipped := make(chan models.BlockID, 1)

	tf.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		blockScheduled <- block.ID()
	})
	tf.Scheduler.Events.BlockSkipped.Hook(func(block *Block) {
		blockSkipped <- block.ID()
	})

	tf.Scheduler.Start()

	// create a new block from a different node and mark it as ready and confirmed, but younger than 1 minute
	blkReadyConfirmedNew := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkReadyConfirmedNew.ID(): types.Void,
	})

	require.NoError(t, tf.Scheduler.Submit(blkReadyConfirmedNew))
	tf.Scheduler.Ready(blkReadyConfirmedNew)

	require.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			require.Equal(t, blkReadyConfirmedNew.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as unready and confirmed, but younger than 1 minute
	blkUnreadyConfirmedNew := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))

	require.NoError(t, tf.Scheduler.Submit(blkUnreadyConfirmedNew))

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkUnreadyConfirmedNew.ID(): types.Void,
	})

	tf.mockAcceptance.Events().BlockAccepted.Trigger(blockgadget.NewBlock(blkUnreadyConfirmedNew.Block, blockgadget.WithAccepted(true)))

	// make sure that the block was not unsubmitted
	require.Equal(t, tf.Scheduler.buffer.IssuerQueue(tf.Issuer("peer").ID()).IDs()[0], blkUnreadyConfirmedNew.ID())
	tf.Scheduler.Ready(blkUnreadyConfirmedNew)

	require.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			require.Equal(t, blkUnreadyConfirmedNew.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as ready and confirmed, but older than 1 minute
	blkReadyConfirmedOld := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(-2*time.Minute)))

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkReadyConfirmedOld.ID(): types.Void,
	})

	require.NoError(t, tf.Scheduler.Submit(blkReadyConfirmedOld))
	tf.Scheduler.Ready(blkReadyConfirmedOld)

	require.Eventually(t, func() bool {
		select {
		case id := <-blockSkipped:
			require.Equal(t, blkReadyConfirmedOld.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as unready and confirmed, but older than 1 minute
	blkUnreadyConfirmedOld := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(-2*time.Minute)))
	require.NoError(t, tf.Scheduler.Submit(blkUnreadyConfirmedOld))
	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
		blkUnreadyConfirmedOld.ID(): types.Void,
	})

	tf.mockAcceptance.Events().BlockAccepted.Trigger(blockgadget.NewBlock(blkUnreadyConfirmedOld.Block, blockgadget.WithAccepted(true)))

	require.Eventually(t, func() bool {
		select {
		case id := <-blockSkipped:
			require.Equal(t, blkUnreadyConfirmedOld.ID(), id)
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_Time(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.CreateIssuer("peer", 10)

	blockScheduled := make(chan *Block, 1)
	tf.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		blockScheduled <- block
	})

	tf.Scheduler.Start()

	futureBlock := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(1*time.Second)))
	require.NoError(t, tf.Scheduler.Submit(futureBlock))

	nowBlock := tf.CreateSchedulerBlock(models.WithIssuer(tf.Issuer("peer").PublicKey()))
	require.NoError(t, tf.Scheduler.Submit(nowBlock))

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
				require.Truef(t, time.Now().After(block.IssuingTime()), "scheduled too early: %s", time.Until(block.IssuingTime()))
				scheduledIDs.Add(block.ID())
			}
		}
	}()

	<-done
	require.Equal(t, models.NewBlockIDs(nowBlock.ID(), futureBlock.ID()), scheduledIDs)
}

func TestScheduler_Issue(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.CreateIssuer("peer", 10)

	tf.Scheduler.Events.Error.Hook(func(err error) {
		require.Failf(t, "unexpected error", "error event triggered: %v", err)
	})

	// setup tangle up till the Scheduler
	tf.Scheduler.Start()

	const numBlocks = 5
	blockScheduled := make(chan models.BlockID, numBlocks)
	tf.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		blockScheduled <- block.ID()
	})

	ids := models.NewBlockIDs()
	for i := 0; i < numBlocks; i++ {
		block := tf.Tangle.BlockDAG.CreateBlock(fmt.Sprintf("blk-%d", i), models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithStrongParents(models.NewBlockIDs(tf.Tangle.BlockDAG.Block("Genesis").ID())))
		ids.Add(block.ID())
		_, _, err := tf.Tangle.Instance.BlockDAG().Attach(block)
		require.NoError(t, err)
	}

	scheduledIDs := models.NewBlockIDs()
	require.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			scheduledIDs.Add(id)
			return len(scheduledIDs) == len(ids)
		default:
			return false
		}
	}, 10*time.Second, 10*time.Millisecond)
	require.Equal(t, ids, scheduledIDs)

	for i := 0; i < numBlocks; i++ {
		block, _ := tf.Scheduler.Block(tf.Tangle.BlockDAG.Block(fmt.Sprintf("blk-%d", i)).ID())
		lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models.BlockID]types.Empty{
			block.ID(): types.Void,
		})
		tf.mockAcceptance.Events().BlockAccepted.Trigger(blockgadget.NewBlock(block.Block, blockgadget.WithAccepted(true)))
	}

	tf.AssertBlocksSkipped(0)
	tf.AssertBlocksScheduled(numBlocks)
}

func TestSchedulerFlow(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.CreateIssuer("peer", 10)
	tf.CreateIssuer("self", 10)

	tf.Scheduler.Events.Error.Hook(func(err error) {
		require.Failf(t, "unexpected error", "error event triggered: %v", err)
	})

	tf.Scheduler.Start()

	// testing desired scheduled order: A - B - D - E - C
	tf.Tangle.BlockDAG.CreateBlock("A", models.WithIssuer(tf.Issuer("self").PublicKey()), models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis")))
	tf.Tangle.BlockDAG.CreateBlock("B", models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(1*time.Second)), models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis")))

	// set C to have a timestamp in the future
	tf.Tangle.BlockDAG.CreateBlock("C", models.WithIssuer(tf.Issuer("self").PublicKey()), models.WithIssuingTime(time.Now().Add(5*time.Second)), models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("A", "B")))

	tf.Tangle.BlockDAG.CreateBlock("D", models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithIssuingTime(time.Now().Add(1*time.Second)), models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("A", "B")))

	tf.Tangle.BlockDAG.CreateBlock("E", models.WithIssuer(tf.Issuer("self").PublicKey()), models.WithIssuingTime(time.Now().Add(3*time.Second)), models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("A", "B")))

	blockScheduled := make(chan models.BlockID, 5)
	tf.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		blockScheduled <- block.ID()
	})

	tf.Tangle.BlockDAG.IssueBlocks("A", "B", "C", "D", "E")
	workers.WaitChildren()

	var scheduledIDs []models.BlockID
	require.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			scheduledIDs = append(scheduledIDs, id)
			return len(scheduledIDs) == 5
		default:
			return false
		}
	}, 10*time.Second, 100*time.Millisecond)

	require.Equal(t, scheduledIDs, []models.BlockID{tf.Tangle.BlockDAG.Block("A").ID(), tf.Tangle.BlockDAG.Block("B").ID(), tf.Tangle.BlockDAG.Block("D").ID(), tf.Tangle.BlockDAG.Block("E").ID(), tf.Tangle.BlockDAG.Block("C").ID()})
}

func TestSchedulerParallelSubmit(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	const totalBlkCount = 200

	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t, workers.CreateGroup("SchedulerTestFramework"))

	tf.Scheduler.Events.Error.Hook(func(err error) {
		require.Failf(t, "unexpected error", "error event triggered: %v", err)
	})

	tf.CreateIssuer("self", 10)
	tf.CreateIssuer("peer", 10)

	tf.Scheduler.Start()

	// generate the blocks we want to solidify
	blockAliases := make([]string, 0, totalBlkCount)

	for i := 0; i < totalBlkCount/2; i++ {
		alias := fmt.Sprintf("blk-%d", i)

		parentOption := models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis"))
		if i > 1 {
			parentOption = models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(fmt.Sprintf("blk-%d", i-1), fmt.Sprintf("blk-%d", i-2)))
		}

		tf.Tangle.BlockDAG.CreateBlock(alias, models.WithIssuer(tf.Issuer("self").PublicKey()), parentOption)
		blockAliases = append(blockAliases, alias)
	}

	for i := totalBlkCount / 2; i < totalBlkCount; i++ {
		alias := fmt.Sprintf("blk-%d", i)

		tf.Tangle.BlockDAG.CreateBlock(alias, models.WithIssuer(tf.Issuer("peer").PublicKey()), models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(fmt.Sprintf("blk-%d", i-1), fmt.Sprintf("blk-%d", i-2))))
		blockAliases = append(blockAliases, alias)
	}

	// issue tips to start solidification
	tf.Tangle.BlockDAG.IssueBlocks(blockAliases...)
	workers.WaitChildren()

	// wait for all blocks to have a formed opinion
	require.Eventually(t, func() bool { return atomic.LoadUint32(&(tf.scheduledBlocksCount)) == totalBlkCount }, 5*time.Minute, 100*time.Millisecond)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
