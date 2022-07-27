package tangleold

import (
	"context"
	"strconv"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/labstack/gommon/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/pow"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

func BenchmarkBlockParser_ParseBytesSame(b *testing.B) {
	blkBytes := lo.PanicOnErr(tangle.NewTestDataBlock("Test").Bytes())
	blkParser := NewParser()
	blkParser.Setup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		blkParser.Parse(blkBytes, nil)
	}
}

func BenchmarkBlockParser_ParseBytesDifferent(b *testing.B) {
	blockBytes := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		blockBytes[i] = lo.PanicOnErr(tangle.NewTestDataBlock("Test" + strconv.Itoa(i)).Bytes())
	}

	blkParser := NewParser()
	blkParser.Setup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		blkParser.Parse(blockBytes[i], nil)
	}
}

func TestBlockParser_ParseBlock(t *testing.T) {
	blk := tangle.NewTestDataBlock("Test")

	blkParser := NewParser()
	blkParser.Setup()
	blkParser.Parse(lo.PanicOnErr(blk.Bytes()), nil)

	blkParser.Events.BlockParsed.Hook(event.NewClosure(func(_ *BlockParsedEvent) {
		log.Infof("parsed block")
	}))
}

var (
	testPeer       *peer.Peer
	testWorker     = pow.New(1)
	testDifficulty = 10
)

//func TestTransactionFilter_Filter(t *testing.T) {
//	filter := NewTransactionFilter()
//	// set callbacks
//	m := &blockCallbackMock{}
//	filter.OnAccept(m.Accept)
//	filter.OnReject(m.Reject)
//
//	t.Run("skip non-transaction payloads", func(t *testing.T) {
//		blk := &Block{}
//		blk.Init()
//
//		blk.payload = payload.NewGenericDataPayload([]byte("hello world"))
//		m.On("Accept", blk, testPeer)
//		filter.Filter(blk, testPeer)
//	})
//}

//func Test_isBlockAndTransactionTimestampsValid(t *testing.T) {
//	blk := &tangle.Block{}
//	blk.Init()
//
//	t.Run("older tx timestamp within limit", func(t *testing.T) {
//		tx := newTransaction(time.Now())
//		blk.M.IssuingTime = tx.Essence().Timestamp().Add(1 * time.Second)
//		assert.True(t, isBlockAndTransactionTimestampsValid(tx, blk))
//	})
//	t.Run("older timestamp but older than max", func(t *testing.T) {
//		tx := newTransaction(time.Now())
//		blk.M.IssuingTime = tx.Essence().Timestamp().Add(MaxReattachmentTimeMin).Add(1 * time.Millisecond)
//		assert.False(t, isBlockAndTransactionTimestampsValid(tx, blk))
//	})
//	t.Run("equal tx and blk timestamp", func(t *testing.T) {
//		tx := newTransaction(time.Now())
//		blk.M.IssuingTime = tx.Essence().Timestamp()
//		assert.True(t, isBlockAndTransactionTimestampsValid(tx, blk))
//	})
//	t.Run("older block", func(t *testing.T) {
//		tx := newTransaction(time.Now())
//		blk.M.IssuingTime = tx.Essence().Timestamp().Add(-1 * time.Millisecond)
//		assert.False(t, isBlockAndTransactionTimestampsValid(tx, blk))
//	})
//}

func TestPowFilter_Filter(t *testing.T) {
	filter := NewPowFilter(testWorker, testDifficulty)

	// set callbacks
	m := &bytesCallbackMock{}
	filter.OnAccept(m.Accept)
	filter.OnReject(m.Reject)

	t.Run("reject small block", func(t *testing.T) {
		m.On("Reject", mock.Anything, mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrBlockTooSmall) }), testPeer)
		filter.Filter(nil, testPeer)
	})

	blk := tangle.NewTestNonceBlock(0)
	blkBytes := lo.PanicOnErr(blk.Bytes())

	t.Run("reject invalid nonce", func(t *testing.T) {
		m.On("Reject", blkBytes, mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrInvalidPOWDifficultly) }), testPeer)
		filter.Filter(blkBytes, testPeer)
	})

	nonce, err := testWorker.Mine(context.Background(), blkBytes[:len(blkBytes)-len(blk.Signature())-pow.NonceBytes], testDifficulty)
	require.NoError(t, err)

	blkPOW := tangle.NewTestNonceBlock(nonce)
	blkPOWBytes := lo.PanicOnErr(blkPOW.Bytes())

	t.Run("accept valid nonce", func(t *testing.T) {
		zeroes, err := testWorker.LeadingZeros(blkPOWBytes[:len(blkPOWBytes)-len(blkPOW.Signature())])
		require.NoError(t, err)
		require.GreaterOrEqual(t, zeroes, testDifficulty)

		m.On("Accept", blkPOWBytes, testPeer)
		filter.Filter(blkPOWBytes, testPeer)
	})

	m.AssertExpectations(t)
}

type bytesCallbackMock struct{ mock.Mock }

func (m *bytesCallbackMock) Accept(blk []byte, p *peer.Peer)            { m.Called(blk, p) }
func (m *bytesCallbackMock) Reject(blk []byte, err error, p *peer.Peer) { m.Called(blk, err, p) }
