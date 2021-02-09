package tangle

import (
	"context"
	"crypto"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/gommon/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func BenchmarkMessageParser_ParseBytesSame(b *testing.B) {
	msgBytes := newTestDataMessage("Test").Bytes()
	msgParser := NewParser()
	msgParser.Setup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(msgBytes, nil)
	}
}

func BenchmarkMessageParser_ParseBytesDifferent(b *testing.B) {
	messageBytes := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = newTestDataMessage("Test" + strconv.Itoa(i)).Bytes()
	}

	msgParser := NewParser()
	msgParser.Setup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(messageBytes[i], nil)
	}
}

func TestMessageParser_ParseMessage(t *testing.T) {
	msg := newTestDataMessage("Test")

	msgParser := NewParser()
	msgParser.Setup()
	msgParser.Parse(msg.Bytes(), nil)

	msgParser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		log.Infof("parsed message")
	}))
}

var (
	testPeer       *peer.Peer
	testWorker     = pow.New(crypto.BLAKE2b_512, 1)
	testDifficulty = 10
)

func TestTransactionFilter_Filter(t *testing.T) {
	filter := NewTransactionFilter()
	// set callbacks
	m := &messageCallbackMock{}
	filter.OnAccept(m.Accept)
	filter.OnReject(m.Reject)

	t.Run("skip non-transaction payloads", func(t *testing.T) {
		msg := &Message{}
		msg.payload = payload.NewGenericDataPayload([]byte("hello world"))
		m.On("Accept", msg, testPeer)
		filter.Filter(msg, testPeer)
	})

	t.Run("reject on failed parse", func(t *testing.T) {
		msg := &Message{payload: &testTxPayload{}}
		m.On("Reject", msg, mock.MatchedBy(func(err error) bool { return err != nil }), testPeer)
		filter.Filter(msg, testPeer)
	})
}

func Test_isMessageAndTransactionTimestampsValid(t *testing.T) {
	msg := &Message{}
	t.Run("older tx timestamp within limit", func(t *testing.T) {
		tx := newTransaction(time.Now())
		msg.issuingTime = tx.Essence().Timestamp().Add(1 * time.Second)
		assert.True(t, isMessageAndTransactionTimestampsValid(tx, msg))
	})
	t.Run("older timestamp but older than max", func(t *testing.T) {
		tx := newTransaction(time.Now())
		msg.issuingTime = tx.Essence().Timestamp().Add(MaxReattachmentTimeMin).Add(1 * time.Millisecond)
		assert.False(t, isMessageAndTransactionTimestampsValid(tx, msg))
	})
	t.Run("equal tx and msg timestamp", func(t *testing.T) {
		tx := newTransaction(time.Now())
		msg.issuingTime = tx.Essence().Timestamp()
		assert.True(t, isMessageAndTransactionTimestampsValid(tx, msg))
	})
	t.Run("older message", func(t *testing.T) {
		tx := newTransaction(time.Now())
		msg.issuingTime = tx.Essence().Timestamp().Add(-1 * time.Millisecond)
		assert.False(t, isMessageAndTransactionTimestampsValid(tx, msg))
	})
}

func TestPowFilter_Filter(t *testing.T) {
	filter := NewPowFilter(testWorker, testDifficulty)

	// set callbacks
	m := &bytesCallbackMock{}
	filter.OnAccept(m.Accept)
	filter.OnReject(m.Reject)

	t.Run("reject small message", func(t *testing.T) {
		m.On("Reject", mock.Anything, mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrMessageTooSmall) }), testPeer)
		filter.Filter(nil, testPeer)
	})

	msg := newTestNonceMessage(0)
	msgBytes := msg.Bytes()

	t.Run("reject invalid nonce", func(t *testing.T) {
		m.On("Reject", msgBytes, mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrInvalidPOWDifficultly) }), testPeer)
		filter.Filter(msgBytes, testPeer)
	})

	nonce, err := testWorker.Mine(context.Background(), msgBytes[:len(msgBytes)-len(msg.Signature())-pow.NonceBytes], testDifficulty)
	require.NoError(t, err)

	msgPOW := newTestNonceMessage(nonce)
	msgPOWBytes := msgPOW.Bytes()

	t.Run("accept valid nonce", func(t *testing.T) {
		zeroes, err := testWorker.LeadingZeros(msgPOWBytes[:len(msgPOWBytes)-len(msgPOW.Signature())])
		require.NoError(t, err)
		require.GreaterOrEqual(t, zeroes, testDifficulty)

		m.On("Accept", msgPOWBytes, testPeer)
		filter.Filter(msgPOWBytes, testPeer)
	})

	m.AssertExpectations(t)
}

type bytesCallbackMock struct{ mock.Mock }

func (m *bytesCallbackMock) Accept(msg []byte, p *peer.Peer)            { m.Called(msg, p) }
func (m *bytesCallbackMock) Reject(msg []byte, err error, p *peer.Peer) { m.Called(msg, err, p) }

type messageCallbackMock struct{ mock.Mock }

func (m *messageCallbackMock) Accept(msg *Message, p *peer.Peer)            { m.Called(msg, p) }
func (m *messageCallbackMock) Reject(msg *Message, err error, p *peer.Peer) { m.Called(msg, err, p) }

type testTxPayload struct{}

func (p *testTxPayload) Type() payload.Type { return ledgerstate.TransactionType }
func (p *testTxPayload) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(32) //random payload size
	marshalUtil.WriteUint32(1337)
	return marshalUtil.Bytes()
}
func (p *testTxPayload) String() string { return "tx" }

func newTransaction(t time.Time) *ledgerstate.Transaction {
	ID, _ := identity.RandomID()
	var inputs ledgerstate.Inputs
	var outputs ledgerstate.Outputs
	essence := ledgerstate.NewTransactionEssence(1, t, ID, ID, inputs, outputs)
	var unlockBlocks ledgerstate.UnlockBlocks
	return ledgerstate.NewTransaction(essence, unlockBlocks)
}
