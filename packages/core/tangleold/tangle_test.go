package tangleold

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/randommap"

	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/consensus/otv"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/pow"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

func BenchmarkVerifyDataBlocks(b *testing.B) {
	tangle := NewTestTangle()

	pool := workerpool.NewBlockingQueuedWorkerPool(workerpool.WorkerCount(runtime.GOMAXPROCS(0)))

	factory := NewBlockFactory(tangle, TipSelectorFunc(func(p payload.Payload, countParents int) (parents BlockIDs) {
		return NewBlockIDs(EmptyBlockID)
	}), emptyLikeReferences)

	blocks := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		blk, err := factory.IssuePayload(payload.NewGenericDataPayload([]byte("some data")))
		require.NoError(b, err)
		blocks[i] = lo.PanicOnErr(blk.Bytes())
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		currentIndex := i
		pool.Submit(func() {
			var blk *Block
			if err := blk.FromBytes(blocks[currentIndex]); err != nil {
				b.Error(err)
			} else {
				if _, err := blk.VerifySignature(); err != nil {
					b.Error(err)
				}
			}
		})
	}

	pool.Stop()
}

func BenchmarkVerifySignature(b *testing.B) {
	tangle := NewTestTangle()

	pool, _ := ants.NewPool(80, ants.WithNonblocking(false))

	factory := NewBlockFactory(tangle, TipSelectorFunc(func(p payload.Payload, countStrongParents int) (parents BlockIDs) {
		return NewBlockIDs(EmptyBlockID)
	}), emptyLikeReferences)

	blocks := make([]*Block, b.N)
	for i := 0; i < b.N; i++ {
		blk, err := factory.IssuePayload(payload.NewGenericDataPayload([]byte("some data")))
		require.NoError(b, err)
		blocks[i] = blk
		blocks[i].Bytes()
	}
	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		currentIndex := i
		if err := pool.Submit(func() {
			blocks[currentIndex].VerifySignature()
			wg.Done()
		}); err != nil {
			b.Error(err)
			return
		}
	}

	wg.Wait()
}

func BenchmarkTangle_StoreBlock(b *testing.B) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	blockBytes := make([]*Block, b.N)
	for i := 0; i < b.N; i++ {
		blockBytes[i] = newTestDataBlock("some data")
		blockBytes[i].Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tangle.Storage.StoreBlock(blockBytes[i])
	}
}

func TestTangle_InvalidParentsAgeBlock(t *testing.T) {
	blockTangle := NewTestTangle()
	blockTangle.Storage.Setup()
	blockTangle.Solidifier.Setup()
	defer blockTangle.Shutdown()

	var storedBlocks, solidBlocks, invalidBlocks int32

	newOldParentsBlock := func(strongParents BlockIDs) *Block {
		block, err := NewBlockWithValidation(emptyLikeReferencesFromStrongParents(strongParents), time.Now().Add(maxParentsTimeDifference+5*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Old")), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))
		assert.NoError(t, err)
		if err := block.DetermineID(); err != nil {
			panic(err)
		}
		return block
	}
	newYoungParentsBlock := func(strongParents BlockIDs) *Block {
		block, err := NewBlockWithValidation(emptyLikeReferencesFromStrongParents(strongParents), time.Now().Add(-maxParentsTimeDifference-5*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Young")), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))
		assert.NoError(t, err)
		if err := block.DetermineID(); err != nil {
			panic(err)
		}
		return block
	}
	newValidBlock := func(strongParents BlockIDs) *Block {
		block, err := NewBlockWithValidation(emptyLikeReferencesFromStrongParents(strongParents), time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("IsBooked")), 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))
		assert.NoError(t, err)
		if err := block.DetermineID(); err != nil {
			panic(err)
		}
		return block
	}

	var wg sync.WaitGroup
	blockTangle.Storage.Events.BlockStored.Hook(event.NewClosure(func(event *BlockStoredEvent) {
		atomic.AddInt32(&storedBlocks, 1)
		wg.Done()
	}))

	blockTangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		atomic.AddInt32(&solidBlocks, 1)
	}))

	blockTangle.Events.BlockInvalid.Hook(event.NewClosure(func(event *BlockInvalidEvent) {
		atomic.AddInt32(&invalidBlocks, 1)
	}))

	blockA := newTestDataBlock("some data")
	blockB := newTestDataBlock("some data1")
	blockC := newValidBlock(NewBlockIDs(blockA.ID(), blockB.ID()))
	blockOldParents := newOldParentsBlock(NewBlockIDs(blockA.ID(), blockB.ID()))
	blockYoungParents := newYoungParentsBlock(NewBlockIDs(blockA.ID(), blockB.ID()))

	wg.Add(5)
	blockTangle.Storage.StoreBlock(blockA)
	blockTangle.Storage.StoreBlock(blockB)
	blockTangle.Storage.StoreBlock(blockC)
	blockTangle.Storage.StoreBlock(blockOldParents)
	blockTangle.Storage.StoreBlock(blockYoungParents)

	// wait for all blocks to become solid
	wg.Wait()

	assert.EqualValues(t, 5, atomic.LoadInt32(&storedBlocks))
	assert.EqualValues(t, 3, atomic.LoadInt32(&solidBlocks))
	assert.EqualValues(t, 2, atomic.LoadInt32(&invalidBlocks))
}

func TestTangle_StoreBlock(t *testing.T) {
	blockTangle := NewTestTangle()
	defer blockTangle.Shutdown()
	if err := blockTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	blockTangle.Storage.Events.BlockStored.Hook(event.NewClosure(func(event *BlockStoredEvent) {
		fmt.Println("STORED:", event.Block.ID())
	}))

	blockTangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		fmt.Println("SOLID:", event.Block.ID())
	}))

	blockTangle.Solidifier.Events.BlockMissing.Hook(event.NewClosure(func(event *BlockMissingEvent) {
		fmt.Println("MISSING:", event.BlockID)
	}))

	blockTangle.Storage.Events.BlockRemoved.Hook(event.NewClosure(func(event *BlockRemovedEvent) {
		fmt.Println("REMOVED:", event.BlockID)
	}))

	newBlockOne := newTestDataBlock("some data")
	newBlockTwo := newTestDataBlock("some other data")

	blockTangle.Storage.StoreBlock(newBlockTwo)

	time.Sleep(7 * time.Second)

	blockTangle.Storage.StoreBlock(newBlockOne)
}

func TestTangle_MissingBlocks(t *testing.T) {
	const (
		blockCount  = 2000
		tangleWidth = 250
		storeDelay  = 5 * time.Millisecond
	)

	// create the tangle
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	tangle.OTVConsensusManager = NewOTVConsensusManager(otv.NewOnTangleVoting(tangle.Ledger.ConflictDAG, tangle.ApprovalWeightManager.WeightOfConflict))

	defer tangle.Shutdown()
	require.NoError(t, tangle.Prune())

	// map to keep track of the tips
	tips := randommap.New[BlockID, BlockID]()
	tips.Set(EmptyBlockID, EmptyBlockID)

	// setup the block factory
	tangle.BlockFactory = NewBlockFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parentsBlockIDs BlockIDs) {
			r := tips.RandomUniqueEntries(countParents)
			if len(r) == 0 {
				return NewBlockIDs(EmptyBlockID)
			}
			parents := NewBlockIDs()
			for _, tip := range r {
				parents.Add(tip)
			}
			return parents
		}),
		emptyLikeReferences,
	)

	// create a helper function that creates the blocks
	createNewBlock := func() *Block {
		// issue the payload
		blk, err := tangle.BlockFactory.IssuePayload(payload.NewGenericDataPayload([]byte("")))
		require.NoError(t, err)

		// remove a tip if the width of the tangle is reached
		if tips.Size() >= tangleWidth {
			tips.Delete(blk.ParentsByType(StrongParentType).First())
		}

		// add current block as a tip
		tips.Set(blk.ID(), blk.ID())

		// return the constructed block
		return blk
	}

	// generate the blocks we want to solidify
	blocks := make(map[BlockID]*Block, blockCount)
	for i := 0; i < blockCount; i++ {
		blk := createNewBlock()
		blocks[blk.ID()] = blk
	}

	// manually set up Tangle flow as far as we need it
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()

	// counter for the different stages
	var (
		storedBlocks  int32
		missingBlocks int32
		solidBlocks   int32
	)
	tangle.Storage.Events.BlockStored.Hook(event.NewClosure(func(_ *BlockStoredEvent) {
		n := atomic.AddInt32(&storedBlocks, 1)
		t.Logf("stored blocks %d/%d", n, blockCount)
	}))

	// increase the counter when a missing block was detected
	tangle.Solidifier.Events.BlockMissing.Hook(event.NewClosure(func(event *BlockMissingEvent) {
		atomic.AddInt32(&missingBlocks, 1)
		// store the block after it has been requested
		go func() {
			time.Sleep(storeDelay)
			tangle.Storage.StoreBlock(blocks[event.BlockID])
		}()
	}))

	// decrease the counter when a missing block was received
	tangle.Storage.Events.MissingBlockStored.Hook(event.NewClosure(func(_ *MissingBlockStoredEvent) {
		n := atomic.AddInt32(&missingBlocks, -1)
		t.Logf("missing blocks %d", n)
	}))

	tangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(_ *BlockSolidEvent) {
		n := atomic.AddInt32(&solidBlocks, 1)
		t.Logf("solid blocks %d/%d", n, blockCount)
	}))

	// issue tips to start solidification
	tips.ForEach(func(key BlockID, _ BlockID) { tangle.Storage.StoreBlock(blocks[key]) })

	// wait for all transactions to become solid
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&solidBlocks) == blockCount }, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValues(t, blockCount, atomic.LoadInt32(&solidBlocks))
	assert.EqualValues(t, blockCount, atomic.LoadInt32(&storedBlocks))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingBlocks))
}

func TestRetrieveAllTips(t *testing.T) {
	blockTangle := NewTestTangle(Identity(selfLocalIdentity))
	blockTangle.Setup()
	defer blockTangle.Shutdown()

	blockA := newTestParentsDataBlock("A", ParentBlockIDs{
		StrongParentType: NewBlockIDs(EmptyBlockID),
	})
	blockB := newTestParentsDataBlock("B", ParentBlockIDs{
		StrongParentType: NewBlockIDs(blockA.ID()),
	})
	blockC := newTestParentsDataBlock("C", ParentBlockIDs{
		StrongParentType: NewBlockIDs(blockA.ID()),
	})

	var wg sync.WaitGroup

	blockTangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(_ *BlockScheduledEvent) {
		wg.Done()
	}))

	wg.Add(3)
	blockTangle.Storage.StoreBlock(blockA)
	blockTangle.Storage.StoreBlock(blockB)
	blockTangle.Storage.StoreBlock(blockC)

	wg.Wait()

	allTips := blockTangle.Storage.RetrieveAllTips()

	assert.Equal(t, 2, len(allTips))
}

func TestTangle_Flow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	const (
		testNetwork = "udp"
		testPort    = 8000
		targetPOW   = 2

		solidBlkCount   = 2000
		invalidBlkCount = 10
		tangleWidth     = 250
		networkDelay    = 5 * time.Millisecond
	)

	var (
		totalBlkCount = solidBlkCount + invalidBlkCount
		testWorker    = pow.New(1)
		// same as gossip manager
		blockWorkerCount     = runtime.GOMAXPROCS(0) * 4
		blockWorkerQueueSize = 1000
	)

	// map to keep track of the tips
	tips := randommap.New[BlockID, BlockID]()
	tips.Set(EmptyBlockID, EmptyBlockID)

	// create the tangle
	tangle := NewTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	// create local peer
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testPort)
	localIdentity := tangle.Options.Identity
	localPeer := peer.NewPeer(localIdentity.Identity, net.IPv4zero, services)

	// set up the block factory
	tangle.BlockFactory = NewBlockFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parentsBlockIDs BlockIDs) {
			r := tips.RandomUniqueEntries(countParents)
			if len(r) == 0 {
				return NewBlockIDs(EmptyBlockID)
			}

			parents := NewBlockIDs()
			for _, tip := range r {
				parents.Add(tip)
			}
			return parents
		}),
		emptyLikeReferences,
	)

	// PoW workers
	tangle.BlockFactory.SetWorker(WorkerFunc(func(blkBytes []byte) (uint64, error) {
		content := blkBytes[:len(blkBytes)-ed25519.SignatureSize-8]
		return testWorker.Mine(context.Background(), content, targetPOW)
	}))
	tangle.BlockFactory.SetTimeout(powTimeout)
	// create a helper function that creates the blocks
	createNewBlock := func(invalidTS bool) *Block {
		var blk *Block
		var err error

		// issue the payload
		if invalidTS {
			blk, err = tangle.BlockFactory.issueInvalidTsPayload(payload.NewGenericDataPayload([]byte("")))
		} else {
			blk, err = tangle.BlockFactory.IssuePayload(payload.NewGenericDataPayload([]byte("")))
		}
		require.NoError(t, err)

		// remove a tip if the width of the tangle is reached
		if !invalidTS {
			if tips.Size() >= tangleWidth {
				tips.Delete(blk.ParentsByType(StrongParentType).First())
			}
		}

		// add current block as a tip
		// only valid block will be in the tip set
		if !invalidTS {
			tips.Set(blk.ID(), blk.ID())
		}
		require.NoError(t, blk.DetermineID())
		// return the constructed block
		return blk
	}

	// setup the block parser
	tangle.Parser.AddBytesFilter(NewPowFilter(testWorker, targetPOW))

	// create inboxWP to act as the gossip layer
	inboxWP := workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		time.Sleep(networkDelay)
		tangle.ProcessGossipBlock(task.Param(0).([]byte), task.Param(1).(*peer.Peer))

		task.Return(nil)
	}, workerpool.WorkerCount(blockWorkerCount), workerpool.QueueSize(blockWorkerQueueSize))
	defer inboxWP.Stop()

	// generate the blocks we want to solidify
	blocks := make(map[BlockID]*Block, solidBlkCount)
	for i := 0; i < solidBlkCount; i++ {
		blk := createNewBlock(false)
		blocks[blk.ID()] = blk
	}

	// generate the invalid timestamp blocks
	invalidblks := make(map[BlockID]*Block, invalidBlkCount)
	for i := 0; i < invalidBlkCount; i++ {
		blk := createNewBlock(true)
		blocks[blk.ID()] = blk
		invalidblks[blk.ID()] = blk
	}

	// counter for the different stages
	var (
		parsedBlocks    int32
		storedBlocks    int32
		missingBlocks   int32
		solidBlocks     int32
		scheduledBlocks int32
		bookedBlocks    int32
		awBlocks        int32
		invalidBlocks   int32
		rejectedBlocks  int32
	)

	tangle.Parser.Events.BytesRejected.Hook(event.NewClosure(func(event *BytesRejectedEvent) {
		t.Logf("rejected bytes %v - %s", event.Bytes, event.Error)
	}))

	// filter rejected events
	tangle.Parser.Events.BlockRejected.Hook(event.NewClosure(func(event *BlockRejectedEvent) {
		n := atomic.AddInt32(&rejectedBlocks, 1)
		t.Logf("rejected by block filter blocks %d/%d - %s %s", n, totalBlkCount, event.Block.ID(), event.Error)
	}))

	tangle.Parser.Events.BlockParsed.Hook(event.NewClosure(func(blkParsedEvent *BlockParsedEvent) {
		n := atomic.AddInt32(&parsedBlocks, 1)
		t.Logf("parsed blocks %d/%d - %s", n, totalBlkCount, blkParsedEvent.Block.ID())
	}))

	// block invalid events
	tangle.Events.BlockInvalid.Hook(event.NewClosure(func(blockInvalidEvent *BlockInvalidEvent) {
		n := atomic.AddInt32(&invalidBlocks, 1)
		t.Logf("invalid blocks %d/%d - %s", n, totalBlkCount, blockInvalidEvent.BlockID)
	}))

	tangle.Storage.Events.BlockStored.Hook(event.NewClosure(func(event *BlockStoredEvent) {
		n := atomic.AddInt32(&storedBlocks, 1)
		t.Logf("stored blocks %d/%d - %s", n, totalBlkCount, event.Block.ID())
	}))

	// increase the counter when a missing block was detected
	tangle.Solidifier.Events.BlockMissing.Hook(event.NewClosure(func(event *BlockMissingEvent) {
		atomic.AddInt32(&missingBlocks, 1)

		// push the block into the gossip inboxWP
		inboxWP.TrySubmit(lo.PanicOnErr(blocks[event.BlockID].Bytes()), localPeer)
	}))

	// decrease the counter when a missing block was received
	tangle.Storage.Events.MissingBlockStored.Hook(event.NewClosure(func(event *MissingBlockStoredEvent) {
		n := atomic.AddInt32(&missingBlocks, -1)
		t.Logf("missing blocks %d - %s", n, event.BlockID)
	}))

	tangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		n := atomic.AddInt32(&solidBlocks, 1)
		t.Logf("solid blocks %d/%d - %s", n, totalBlkCount, event.Block.ID())
	}))

	tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		n := atomic.AddInt32(&scheduledBlocks, 1)
		t.Logf("scheduled blocks %d/%d - %s", n, totalBlkCount, event.BlockID)
	}))

	tangle.Booker.Events.BlockBooked.Hook(event.NewClosure(func(event *BlockBookedEvent) {
		n := atomic.AddInt32(&bookedBlocks, 1)
		t.Logf("booked blocks %d/%d - %s", n, totalBlkCount, event.BlockID)
	}))

	tangle.ApprovalWeightManager.Events.BlockProcessed.Hook(event.NewClosure(func(*BlockProcessedEvent) {
		n := atomic.AddInt32(&awBlocks, 1)
		t.Logf("approval weight processed blocks %d/%d", n, totalBlkCount)
	}))

	tangle.Events.Error.Hook(event.NewClosure(func(err error) {
		t.Logf("Error %s", err)
	}))

	// setup data flow
	tangle.Setup()

	// issue tips to start solidification
	tips.ForEach(func(key BlockID, _ BlockID) {
		if key == EmptyBlockID {
			return
		}
		inboxWP.TrySubmit(lo.PanicOnErr(blocks[key].Bytes()), localPeer)
	})
	// incoming invalid blocks
	for _, blk := range invalidblks {
		inboxWP.TrySubmit(lo.PanicOnErr(blk.Bytes()), localPeer)
	}

	// wait for all blocks to be scheduled
	lastWaitNotice := time.Now()
	assert.Eventually(t, func() bool {
		if time.Now().Sub(lastWaitNotice) > time.Second {
			lastWaitNotice = time.Now()
			t.Logf("waiting for scheduled blocks %d/%d", atomic.LoadInt32(&scheduledBlocks), totalBlkCount)
		}

		return atomic.LoadInt32(&scheduledBlocks) == solidBlkCount
	}, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValuesf(t, totalBlkCount, atomic.LoadInt32(&parsedBlocks), "parsed blocks does not match")
	assert.EqualValuesf(t, totalBlkCount, atomic.LoadInt32(&storedBlocks), "stored blocks does not match")
	assert.EqualValues(t, solidBlkCount, atomic.LoadInt32(&solidBlocks))
	assert.EqualValues(t, solidBlkCount, atomic.LoadInt32(&scheduledBlocks))
	assert.EqualValues(t, solidBlkCount, atomic.LoadInt32(&bookedBlocks))
	assert.EqualValues(t, solidBlkCount, atomic.LoadInt32(&awBlocks))
	// blocks with invalid timestamp are not forwarded from the timestamp filter, thus there are 0.
	assert.EqualValues(t, invalidBlkCount, atomic.LoadInt32(&invalidBlocks))
	assert.EqualValues(t, 0, atomic.LoadInt32(&rejectedBlocks))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingBlocks))
}

// IssueInvalidTsPayload creates a new block including sequence number and tip selection and returns it.
func (f *BlockFactory) issueInvalidTsPayload(p payload.Payload, _ ...*Tangle) (*Block, error) {
	payloadLen := len(lo.PanicOnErr(p.Bytes()))
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	sequenceNumber, err := f.sequence.Next()
	if err != nil {
		err = fmt.Errorf("could not create sequence number: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	parents := f.selector.Tips(p, 2)
	if err != nil {
		err = fmt.Errorf("could not select tips: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	issuingTime := time.Now().Add(maxParentsTimeDifference + 5*time.Minute)
	issuerPublicKey := f.localIdentity.PublicKey()

	// do the PoW
	nonce, err := f.doPOW(emptyLikeReferencesFromStrongParents(parents), issuingTime, issuerPublicKey, sequenceNumber, p, 0, epoch.NewECRecord(0))
	if err != nil {
		err = fmt.Errorf("pow failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// create the signature
	signature, err := f.sign(emptyLikeReferencesFromStrongParents(parents), issuingTime, issuerPublicKey, sequenceNumber, p, nonce, 0, epoch.NewECRecord(0))
	if err != nil {
		err = fmt.Errorf("signing failed failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	blk, err := NewBlockWithValidation(
		emptyLikeReferencesFromStrongParents(parents),
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
		0,
		epoch.NewECRecord(0),
	)
	if err != nil {
		err = fmt.Errorf("problem with block syntax: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}
	return blk, nil
}
