package tangleold

import (
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/schedulerutils"
)

// region Scheduler_test /////////////////////////////////////////////////////////////////////////////////////////////

func TestScheduler_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	tangle.Scheduler.Start()

	time.Sleep(10 * time.Millisecond)
	tangle.Scheduler.Shutdown()
}

func TestScheduler_Submit(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	tangle.Scheduler.Start()

	blk := newBlock(selfNode.PublicKey())
	tangle.Storage.StoreBlock(blk)
	assert.NoError(t, tangle.Scheduler.Submit(blk.ID()))
	time.Sleep(100 * time.Millisecond)
	// unsubmit to allow the scheduler to shutdown
	assert.NoError(t, tangle.Scheduler.Unsubmit(blk.ID()))
}

func TestScheduler_updateActiveNodeList(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	nodes := make(map[string]*identity.Identity)

	tangle.Scheduler.updateActiveNodesList(map[identity.ID]float64{})
	assert.Equal(t, 0, tangle.Scheduler.buffer.NumActiveNodes())

	for _, node := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		nodes[node] = identity.GenerateIdentity()
	}
	tangle.Scheduler.updateActiveNodesList(map[identity.ID]float64{
		nodes["A"].ID(): 30,
		nodes["B"].ID(): 15,
		nodes["C"].ID(): 25,
		nodes["D"].ID(): 20,
		nodes["E"].ID(): 10,
		nodes["G"].ID(): 0,
	})

	assert.Equal(t, 5, tangle.Scheduler.buffer.NumActiveNodes())
	assert.NotContains(t, tangle.Scheduler.buffer.NodeIDs(), nodes["G"].ID())

	tangle.Scheduler.updateActiveNodesList(map[identity.ID]float64{
		nodes["A"].ID(): 30,
		nodes["B"].ID(): 15,
		nodes["C"].ID(): 25,
		nodes["E"].ID(): 0,
		nodes["F"].ID(): 1,
		nodes["G"].ID(): 5,
	})
	assert.Equal(t, 5, tangle.Scheduler.buffer.NumActiveNodes())
	assert.Contains(t, tangle.Scheduler.buffer.NodeIDs(), nodes["A"].ID())
	assert.Contains(t, tangle.Scheduler.buffer.NodeIDs(), nodes["B"].ID())
	assert.Contains(t, tangle.Scheduler.buffer.NodeIDs(), nodes["C"].ID())
	assert.Contains(t, tangle.Scheduler.buffer.NodeIDs(), nodes["F"].ID())
	assert.Contains(t, tangle.Scheduler.buffer.NodeIDs(), nodes["G"].ID())

	tangle.Scheduler.updateActiveNodesList(map[identity.ID]float64{})
	assert.Equal(t, 0, tangle.Scheduler.buffer.NumActiveNodes())
}

func TestScheduler_Discarded(t *testing.T) {
	t.Skip("Skip test. Zero mana nodes are allowed to issue blocks.")
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	noAManaNode := identity.GenerateIdentity()

	blockDiscarded := make(chan BlockID, 1)
	tangle.Scheduler.Events.BlockDiscarded.Hook(event.NewClosure(func(event *BlockDiscardedEvent) {
		blockDiscarded <- event.BlockID
	}))

	tangle.Scheduler.Start()

	// this node has no mana so the block will be discarded
	blk := newBlock(noAManaNode.PublicKey())
	tangle.Storage.StoreBlock(blk)
	err := tangle.Scheduler.Submit(blk.ID())
	assert.Truef(t, errors.Is(err, schedulerutils.ErrInsufficientMana), "unexpected error: %v", err)

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockDiscarded:
			return assert.Equal(t, blk.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_SetRateBeforeStart(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Scheduler.SetRate(time.Hour)
	tangle.Scheduler.Start()
	tangle.Scheduler.SetRate(testRate)
}

func TestScheduler_Schedule(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	blockScheduled := make(chan BlockID, 1)
	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		blockScheduled <- event.BlockID
	}))

	tangle.Scheduler.Start()

	// create a new block from a different node
	blk := newBlock(peerNode.PublicKey())
	tangle.Storage.StoreBlock(blk)
	assert.NoError(t, tangle.Scheduler.Submit(blk.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(blk.ID()))

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			return assert.Equal(t, blk.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

// MockConfirmationOracleConfirmed mocks ConfirmationOracle marking all blocks as confirmed.
type MockConfirmationOracleConfirmed struct {
	ConfirmationOracle
	events *ConfirmationEvents
}

// IsBlockConfirmed mocks its interface function returning that all blocks are confirmed.
func (m *MockConfirmationOracleConfirmed) IsBlockConfirmed(_ BlockID) bool {
	return true
}

func (m *MockConfirmationOracleConfirmed) Events() *ConfirmationEvents {
	return m.events
}

func TestScheduler_SkipConfirmed(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	tangle.ConfirmationOracle = &MockConfirmationOracleConfirmed{
		ConfirmationOracle: tangle.ConfirmationOracle,
		events:             NewConfirmationEvents(),
	}
	blockScheduled := make(chan BlockID, 1)
	blockSkipped := make(chan BlockID, 1)

	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		blockScheduled <- event.BlockID
	}))
	tangle.Scheduler.Events.BlockSkipped.Hook(event.NewClosure(func(event *BlockSkippedEvent) {
		blockSkipped <- event.BlockID
	}))

	tangle.Scheduler.Setup()

	// create a new block from a different node and mark it as ready and confirmed, but younger than 1 minute
	blkReadyConfirmedNew := newBlock(peerNode.PublicKey())
	tangle.Storage.StoreBlock(blkReadyConfirmedNew)
	assert.NoError(t, tangle.Scheduler.Submit(blkReadyConfirmedNew.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(blkReadyConfirmedNew.ID()))
	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			return assert.Equal(t, blkReadyConfirmedNew.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as unready and confirmed, but younger than 1 minute
	blkUnreadyConfirmedNew := newBlock(peerNode.PublicKey())
	tangle.Storage.StoreBlock(blkUnreadyConfirmedNew)
	assert.NoError(t, tangle.Scheduler.Submit(blkUnreadyConfirmedNew.ID()))
	tangle.ConfirmationOracle.Events().BlockAccepted.Trigger(&BlockAcceptedEvent{blkUnreadyConfirmedNew})
	// make sure that the block was not unsubmitted
	assert.Equal(t, blockIDFromElementID(tangle.Scheduler.buffer.NodeQueue(peerNode.ID()).IDs()[0]), blkUnreadyConfirmedNew.ID())
	assert.NoError(t, tangle.Scheduler.Ready(blkUnreadyConfirmedNew.ID()))
	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			return assert.Equal(t, blkUnreadyConfirmedNew.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as ready and confirmed, but older than 1 minute
	blkReadyConfirmedOld := newBlockWithTimestamp(peerNode.PublicKey(), time.Now().Add(-2*time.Minute))
	tangle.Storage.StoreBlock(blkReadyConfirmedOld)
	assert.NoError(t, tangle.Scheduler.Submit(blkReadyConfirmedOld.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(blkReadyConfirmedOld.ID()))
	assert.Eventually(t, func() bool {
		select {
		case id := <-blockSkipped:
			return assert.Equal(t, blkReadyConfirmedOld.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

	// create a new block from a different node and mark it as unready and confirmed, but older than 1 minute
	blkUnreadyConfirmedOld := newBlockWithTimestamp(peerNode.PublicKey(), time.Now().Add(-2*time.Minute))
	tangle.Storage.StoreBlock(blkUnreadyConfirmedOld)
	assert.NoError(t, tangle.Scheduler.Submit(blkUnreadyConfirmedOld.ID()))
	tangle.ConfirmationOracle.Events().BlockAccepted.Trigger(&BlockAcceptedEvent{blkUnreadyConfirmedOld})

	assert.Eventually(t, func() bool {
		select {
		case id := <-blockSkipped:
			return assert.Equal(t, blkUnreadyConfirmedOld.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_SetRate(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	var scheduled atomic.Bool
	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(_ *BlockScheduledEvent) {
		scheduled.Store(true)
	}))

	tangle.Scheduler.Start()

	// effectively disable the scheduler by setting a very low rate
	tangle.Scheduler.SetRate(time.Hour)
	// assure that any potential ticks issued before the rate change have been processed
	time.Sleep(100 * time.Millisecond)

	// submit a new block to the scheduler
	blk := newBlock(peerNode.PublicKey())
	tangle.Storage.StoreBlock(blk)
	assert.NoError(t, tangle.Scheduler.Submit(blk.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(blk.ID()))

	// the block should not be scheduled as the rate is too low
	time.Sleep(100 * time.Millisecond)
	assert.False(t, scheduled.Load())

	// after reducing the rate again, the block should eventually be scheduled
	tangle.Scheduler.SetRate(10 * time.Millisecond)
	assert.Eventually(t, scheduled.Load, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_Time(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	blockScheduled := make(chan BlockID, 1)
	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		blockScheduled <- event.BlockID
	}))

	tangle.Scheduler.Start()

	future := newBlock(peerNode.PublicKey())
	future.M.IssuingTime = time.Now().Add(time.Second)
	tangle.Storage.StoreBlock(future)
	assert.NoError(t, tangle.Scheduler.Submit(future.ID()))

	now := newBlock(peerNode.PublicKey())
	tangle.Storage.StoreBlock(now)
	assert.NoError(t, tangle.Scheduler.Submit(now.ID()))

	assert.NoError(t, tangle.Scheduler.Ready(future.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(now.ID()))

	done := make(chan struct{})
	scheduledIDs := NewBlockIDs()
	go func() {
		defer close(done)
		timer := time.NewTimer(time.Until(future.IssuingTime()) + 100*time.Millisecond)
		for {
			select {
			case <-timer.C:
				return
			case id := <-blockScheduled:
				tangle.Storage.Block(id).Consume(func(blk *Block) {
					assert.Truef(t, time.Now().After(blk.IssuingTime()), "scheduled too early: %s", time.Until(blk.IssuingTime()))
					scheduledIDs.Add(id)
				})
			}
		}
	}()

	<-done
	assert.Equal(t, NewBlockIDs(now.ID(), future.ID()), scheduledIDs)
}

func TestScheduler_Issue(t *testing.T) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Events.Error.Hook(event.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	// setup tangle up till the Scheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Scheduler.Setup()
	tangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		assert.NoError(t, tangle.Scheduler.SubmitAndReady(event.Block))
	}))
	tangle.Scheduler.Start()

	const numBlocks = 5
	blockScheduled := make(chan BlockID, numBlocks)
	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		blockScheduled <- event.BlockID
	}))

	ids := NewBlockIDs()
	for i := 0; i < numBlocks; i++ {
		blk := newBlock(selfNode.PublicKey())
		tangle.Storage.StoreBlock(blk)
		ids.Add(blk.ID())
	}

	scheduledIDs := NewBlockIDs()
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
}

func TestSchedulerFlow(t *testing.T) {
	// create Scheduler dependencies
	// create the tangle
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Events.Error.Hook(event.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	// setup tangle up till the Scheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Scheduler.Setup()
	tangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		assert.NoError(t, tangle.Scheduler.SubmitAndReady(event.Block))
	}))
	tangle.Scheduler.Start()

	// testing desired scheduled order: A - B - D - C  (B - A - D - C is equivalent)
	blocks := make(map[string]*Block)
	blocks["A"] = newBlock(selfNode.PublicKey())
	blocks["B"] = newBlock(peerNode.PublicKey())

	// set C to have a timestamp in the future
	blkC := newBlock(selfNode.PublicKey())

	blkC.M.Parents.AddAll(StrongParentType, NewBlockIDs(blocks["A"].ID(), blocks["B"].ID()))

	blkC.M.IssuingTime = time.Now().Add(5 * time.Second)
	blocks["C"] = blkC

	blkD := newBlock(peerNode.PublicKey())
	blkD.M.Parents.AddAll(StrongParentType, NewBlockIDs(blocks["A"].ID(), blocks["B"].ID()))
	blocks["D"] = blkD

	blkE := newBlock(selfNode.PublicKey())
	blkE.M.Parents.AddAll(StrongParentType, NewBlockIDs(blocks["A"].ID(), blocks["B"].ID()))
	blkE.M.IssuingTime = time.Now().Add(3 * time.Second)
	blocks["E"] = blkE

	blockScheduled := make(chan BlockID, len(blocks))
	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		blockScheduled <- event.BlockID
	}))

	for _, block := range blocks {
		tangle.Storage.StoreBlock(block)
	}

	var scheduledIDs []BlockID
	assert.Eventually(t, func() bool {
		select {
		case id := <-blockScheduled:
			scheduledIDs = append(scheduledIDs, id)
			return len(scheduledIDs) == len(blocks)
		default:
			return false
		}
	}, 10*time.Second, 100*time.Millisecond)
}

func TestSchedulerParallelSubmit(t *testing.T) {
	const (
		totalBlkCount = 200
		tangleWidth   = 250
		networkDelay  = 5 * time.Millisecond
	)

	var totalScheduled atomic.Int32

	// create Scheduler dependencies
	// create the tangle
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Events.Error.Hook(event.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	// setup tangle up till the Scheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Scheduler.Setup()
	tangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		assert.NoError(t, tangle.Scheduler.SubmitAndReady(event.Block))
	}))
	tangle.Scheduler.Start()

	// generate the blocks we want to solidify
	blocks := make(map[BlockID]*Block, totalBlkCount)
	for i := 0; i < totalBlkCount/2; i++ {
		blk := newBlock(selfNode.PublicKey())
		blocks[blk.ID()] = blk
	}

	for i := 0; i < totalBlkCount/2; i++ {
		blk := newBlock(peerNode.PublicKey())
		blocks[blk.ID()] = blk
	}

	tangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		t.Logf(event.Block.ID().Base58(), " solid")
	}))

	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		n := totalScheduled.Add(1)
		t.Logf("scheduled blocks %d/%d", n, totalBlkCount)
	}))

	// issue tips to start solidification
	t.Run("ParallelSubmit", func(t *testing.T) {
		for _, m := range blocks {
			t.Run(m.ID().Base58(), func(t *testing.T) {
				m := m
				t.Parallel()
				t.Logf("issue block: %s", m.ID().Base58())
				tangle.Storage.StoreBlock(m)
			})
		}
	})

	// wait for all blocks to have a formed opinion
	assert.Eventually(t, func() bool { return totalScheduled.Load() == totalBlkCount }, 5*time.Minute, 100*time.Millisecond)
}

func BenchmarkScheduler(b *testing.B) {
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	blk := newBlock(selfNode.PublicKey())
	tangle.Storage.StoreBlock(blk)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := tangle.Scheduler.SubmitAndReady(blk); err != nil {
			b.Fatal(err)
		}
		tangle.Scheduler.schedule()
	}
	b.StopTimer()
}

var (
	timeOffset      = 0 * time.Nanosecond
	timeOffsetMutex = sync.Mutex{}
)

func newBlock(issuerPublicKey ed25519.PublicKey) *Block {
	timeOffsetMutex.Lock()
	timeOffset++
	block := NewBlock(
		emptyLikeReferencesFromStrongParents(NewBlockIDs(EmptyBlockID)),
		time.Now().Add(timeOffset),
		issuerPublicKey,
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
		0,
		epoch.NewECRecord(0),
	)
	timeOffsetMutex.Unlock()
	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

func newBlockWithTimestamp(issuerPublicKey ed25519.PublicKey, timestamp time.Time) *Block {
	block := NewBlock(
		ParentBlockIDs{
			StrongParentType: {
				EmptyBlockID: types.Void,
			},
		},
		timestamp,
		issuerPublicKey,
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
		0,
		epoch.NewECRecord(0),
	)
	if err := block.DetermineID(); err != nil {
		panic(err)
	}
	return block
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
