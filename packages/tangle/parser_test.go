package tangle

import (
	"context"
	"crypto"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/gommon/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
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
	testPeer       = newTestPeer("peerTest")
	testWorker     = pow.New(crypto.BLAKE2b_512, 1)
	testDifficulty = 10
	testWindow     = 60
	testRate       = 1.
)

var (
	testNetwork = "udp"
	testIP      = net.IPv4zero
	testPort    = 8000
)

func newTestServiceRecord() *service.Record {
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testPort)

	return services
}

func newTestPeer(name string) *peer.Peer {
	key := ed25519.PublicKey{}
	copy(key[:], name)
	return peer.NewPeer(identity.New(key), testIP, newTestServiceRecord())
}

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

func TestAPowFilter_Filter(t *testing.T) {
	filter := NewPowFilter(testWorker, testDifficulty)
	pow.BaseDifficulty = testDifficulty
	pow.ApowWindow = testWindow
	pow.AdaptiveRate = testRate

	// set callbacks
	m := &messageCallbackMock{}
	filter.OnAccept(m.Accept)
	filter.OnReject(m.Reject)

	msg := newTestAPOWMessage(time.Now())
	msgBytes := msg.Bytes()
	nonce, err := testWorker.Mine(context.Background(), msgBytes[:len(msgBytes)-len(msg.Signature())-pow.NonceBytes], testDifficulty)
	require.NoError(t, err)

	msg.nonce = nonce
	msg.bytes = nil

	// accpet since apow difficulty should be >= diffficulty
	m.On("Accept", msg, testPeer)
	filter.Filter(msg, testPeer)

	// loop to force the number of zeros to be == testDifficulty
	zeros := 0
	for zeros != testDifficulty {
		msg = newTestAPOWMessage(time.Now())
		msg.bytes = nil
		msgBytes = msg.Bytes()
		nonce, err = testWorker.Mine(context.Background(), msgBytes[:len(msgBytes)-len(msg.Signature())-pow.NonceBytes], testDifficulty)
		require.NoError(t, err)
		zeros, err = testWorker.LeadingZeros(msgBytes[:len(msgBytes)-len(msg.Signature())])
		require.NoError(t, err)
	}

	require.Equal(t, testDifficulty, zeros)

	msg.nonce = nonce
	msg.bytes = nil
	// reject since apow difficulty should be >= diffficulty + 1
	m.On("Reject", msg, mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrInvalidPOWDifficultly) }), testPeer)
	filter.Filter(msg, testPeer)

	nonce, err = testWorker.Mine(context.Background(), msgBytes[:len(msgBytes)-len(msg.Signature())-pow.NonceBytes], testDifficulty+1)
	require.NoError(t, err)

	msg.nonce = nonce
	msg.bytes = nil
	msgBytes = msg.Bytes()

	zeros, err = testWorker.LeadingZeros(msgBytes[:len(msgBytes)-len(msg.Signature())])
	require.NoError(t, err)
	require.GreaterOrEqual(t, zeros, testDifficulty+1)

	// accept since apow difficulty should be >= diffficulty + 1
	m.On("Accept", msg, testPeer)
	filter.Filter(msg, testPeer)

	m.AssertExpectations(t)
}

func TestAPowFilter_Parallel(t *testing.T) {
	testDifficulty = 0
	filter := NewPowFilter(testWorker, testDifficulty)
	pow.BaseDifficulty = testDifficulty
	pow.ApowWindow = 5
	pow.AdaptiveRate = 0.

	// set callbacks
	m := &messageCallbackMock{}
	filter.OnAccept(m.Accept)
	filter.OnReject(m.Reject)

	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			msg := newTestAPOWMessage(time.Now().Add(time.Duration(i) * time.Second))

			m.On("Accept", msg, testPeer)
			filter.Filter(msg, testPeer)
			wg.Done()
		}(i)
	}

	wg.Wait()
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
	marshalUtil.WriteUint32(32) // random payload size
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
