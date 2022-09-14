package blockfactory

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models/payload"
)

func TestFactory_IssuePayload(t *testing.T) {
	localIdentity := identity.GenerateLocalIdentity()

	ecRecord := epoch.NewECRecord(10)
	ecRecord.SetECR([32]byte{123, 255})
	ecRecord.SetPrevEC([32]byte{90, 111})
	confirmedEpochIndex := epoch.Index(25)
	commitmentFunc := func() (*epoch.ECRecord, epoch.Index, error) {
		return ecRecord, confirmedEpochIndex, nil
	}

	var randomBlockID1, randomBlockID2 models.BlockID
	_ = randomBlockID1.FromRandomness()
	_ = randomBlockID2.FromRandomness()
	tipSelectorFunc := func(countParents int) models.BlockIDs {
		return models.NewBlockIDs(randomBlockID1, randomBlockID2)
	}

	pay := payload.NewGenericDataPayload([]byte("test"))

	factory := NewBlockFactory(localIdentity, tipSelectorFunc, commitmentFunc)
	createdBlock, err := factory.IssuePayload(pay, 2)
	require.NoError(t, err)

	assert.Contains(t, createdBlock.ParentsByType(models.StrongParentType), randomBlockID1, randomBlockID2)
	assert.Equal(t, localIdentity.PublicKey(), createdBlock.IssuerPublicKey())
	// issuingTime
	assert.WithinRange(t, createdBlock.IssuingTime(), time.Now().Add(-1*time.Minute), time.Now().Add(1*time.Minute))
	assert.EqualValues(t, 1337, createdBlock.SequenceNumber())
	assert.Equal(t, lo.PanicOnErr(pay.Bytes()), lo.PanicOnErr(createdBlock.Payload().Bytes()))
	assert.Equal(t, ecRecord.EI(), createdBlock.EI())
	assert.Equal(t, ecRecord.ECR(), createdBlock.ECR())
	assert.Equal(t, ecRecord.PrevEC(), createdBlock.PrevEC())
	assert.Equal(t, confirmedEpochIndex, createdBlock.LatestConfirmedEpoch())
	assert.EqualValues(t, 42, createdBlock.Nonce())

	signatureValid, err := createdBlock.VerifySignature()
	require.NoError(t, err)
	assert.True(t, signatureValid)
}

// import (
// 	"context"
// 	"crypto/ed25519"
// 	"fmt"
// 	"sync"
// 	"sync/atomic"
// 	"testing"
// 	"time"
//
// 	"github.com/iotaledger/hive.go/generics/event"
// 	"github.com/iotaledger/hive.go/generics/set"
// 	"github.com/iotaledger/hive.go/identity"
// 	"github.com/iotaledger/hive.go/types"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// 	_ "golang.org/x/crypto/blake2b"
//
// 	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
// 	"github.com/iotaledger/goshimmer/packages/node/clock"
//
// 	"github.com/iotaledger/goshimmer/packages/core/pow"
// 	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
// )
//
// const (
// 	targetPOW   = 10
// 	powTimeout  = 10 * time.Second
// 	totalBlocks = 2000
// )
//
// func TestBlockFactory_BuildBlock(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("skipping test in short mode.")
// 	}
// 	selfLocalIdentity := identity.GenerateLocalIdentity()
// 	tangle := NewTestTangle(Identity(selfLocalIdentity))
// 	defer tangle.Shutdown()
// 	mockOTV := &SimpleMockOnTangleVoting{}
// 	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)
//
// 	tangle.Factory = NewBlockFactory(
// 		tangle,
// 		TipSelectorFunc(func(p payload.Payload, countParents int) (parents BlockIDs) {
// 			return NewBlockIDs(EmptyBlockID)
// 		}),
// 		emptyLikeReferences,
// 	)
// 	tangle.Factory.SetTimeout(powTimeout)
// 	defer tangle.Factory.Shutdown()
//
// 	// keep track of sequence numbers
// 	sequenceNumbers := sync.Map{}
//
// 	// attach to event and count
// 	countEvents := uint64(0)
// 	tangle.Factory.Events.BlockConstructed.Hook(event.NewClosure(func(_ *BlockConstructedEvent) {
// 		atomic.AddUint64(&countEvents, 1)
// 	}))
//
// 	t.Run("CheckProperties", func(t *testing.T) {
// 		p := payload.NewGenericDataPayload([]byte("TestCheckProperties"))
// 		blk, err := tangle.Factory.IssuePayload(p)
// 		require.NoError(t, err)
//
// 		// TODO: approval switch: make test case with weak parents
// 		assert.NotEmpty(t, blk.ParentsByType(StrongParentType))
//
// 		// time in range of 0.1 seconds
// 		assert.InDelta(t, clock.SyncedTime().UnixNano(), blk.IssuingTime().UnixNano(), 100000000)
//
// 		// check payload
// 		assert.Equal(t, p, blk.Payload())
//
// 		// check total events and sequence number
// 		assert.EqualValues(t, 1, countEvents)
// 		assert.EqualValues(t, 0, blk.SequenceNumber())
//
// 		sequenceNumbers.Store(blk.SequenceNumber(), true)
// 	})
//
// 	// create blocks in parallel
// 	t.Run("ParallelCreation", func(t *testing.T) {
// 		for i := 1; i < totalBlocks; i++ {
// 			t.Run("test", func(t *testing.T) {
// 				t.Parallel()
//
// 				p := payload.NewGenericDataPayload([]byte("TestParallelCreation"))
// 				blk, err := tangle.Factory.IssuePayload(p)
// 				require.NoError(t, err)
//
// 				// TODO: approval switch: make test case with weak parents
// 				assert.NotEmpty(t, blk.ParentsByType(StrongParentType))
//
// 				// time in range of 0.1 seconds
// 				assert.InDelta(t, clock.SyncedTime().UnixNano(), blk.IssuingTime().UnixNano(), 100000000)
//
// 				// check payload
// 				assert.Equal(t, p, blk.Payload())
//
// 				sequenceNumbers.Store(blk.SequenceNumber(), true)
// 			})
// 		}
// 	})
//
// 	// check total events and sequence number
// 	assert.EqualValues(t, totalBlocks, countEvents)
//
// 	max := uint64(0)
// 	countSequence := 0
// 	sequenceNumbers.Range(func(key, value interface{}) bool {
// 		seq := key.(uint64)
// 		val := value.(bool)
// 		if val != true {
// 			return false
// 		}
//
// 		// check for max sequence number
// 		if seq > max {
// 			max = seq
// 		}
// 		countSequence++
// 		return true
// 	})
// 	assert.EqualValues(t, totalBlocks-1, max)
// 	assert.EqualValues(t, totalBlocks, countSequence)
// }
//
// func TestBlockFactory_POW(t *testing.T) {
// 	mockOTV := &SimpleMockOnTangleVoting{}
//
// 	tangle := NewTestTangle()
// 	defer tangle.Shutdown()
// 	tangle.OTVConsensusManager = NewOTVConsensusManager(mockOTV)
//
// 	blkFactory := NewBlockFactory(
// 		tangle,
// 		TipSelectorFunc(func(p payload.Payload, countParents int) (parentsBlockIDs BlockIDs) {
// 			return NewBlockIDs(EmptyBlockID)
// 		}),
// 		emptyLikeReferences,
// 	)
// 	defer blkFactory.Shutdown()
//
// 	worker := pow.New(1)
//
// 	blkFactory.SetWorker(WorkerFunc(func(blkBytes []byte) (uint64, error) {
// 		content := blkBytes[:len(blkBytes)-ed25519.SignatureSize-8]
// 		return worker.Mine(context.Background(), content, targetPOW)
// 	}))
// 	blkFactory.SetTimeout(powTimeout)
// 	blk, err := blkFactory.IssuePayload(payload.NewGenericDataPayload([]byte("test")))
// 	require.NoError(t, err)
//
// 	blkBytes, err := blk.Bytes()
// 	require.NoError(t, err)
// 	content := blkBytes[:len(blkBytes)-ed25519.SignatureSize-8]
//
// 	zeroes, err := worker.LeadingZerosWithNonce(content, blk.Nonce())
// 	assert.GreaterOrEqual(t, zeroes, targetPOW)
// 	assert.NoError(t, err)
// }
//
