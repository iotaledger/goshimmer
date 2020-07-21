package builtinfilters

import (
	"context"
	"crypto"
	"errors"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b" // required by crypto.BLAKE2b_512
)

var (
	testPayload    = payload.NewData([]byte("test"))
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

	msg := newTestMessage(0)
	msgBytes := msg.Bytes()

	t.Run("reject invalid nonce", func(t *testing.T) {
		m.On("Reject", msgBytes, mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrInvalidPOWDifficultly) }), testPeer)
		filter.Filter(msgBytes, testPeer)
	})

	nonce, err := testWorker.Mine(context.Background(), msgBytes[:len(msgBytes)-len(msg.Signature())-pow.NonceBytes], testDifficulty)
	require.NoError(t, err)

	msgPOW := newTestMessage(nonce)
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

func newTestMessage(nonce uint64) *message.Message {
	return message.New(message.EmptyId, message.EmptyId, time.Time{}, ed25519.PublicKey{}, 0, testPayload, nonce, ed25519.Signature{})
}
