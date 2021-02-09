package tangle

import (
	"context"
	"crypto"
	"strconv"
	"testing"

	"github.com/go-errors/errors"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/labstack/gommon/log"
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

func TestPowFilter_Filter(t *testing.T) {
	filter := NewPowFilter(testWorker, testDifficulty)

	// set callbacks
	m := &callbackMock{}
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

type callbackMock struct{ mock.Mock }

func (m *callbackMock) Accept(msg []byte, p *peer.Peer)            { m.Called(msg, p) }
func (m *callbackMock) Reject(msg []byte, err error, p *peer.Peer) { m.Called(msg, err, p) }
