package pow

import (
	"errors"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testPayload               = payload.NewData([]byte("test"))
	testPeer       *peer.Peer = nil
	testDifficulty            = 10
)

func init() {
	log = logger.NewExampleLogger("pow_test")
	// set the test parameters
	config.Node.Set(CfgPOWDifficulty, testDifficulty)
	config.Node.Set(CfgPOWNumThreads, 1)
	config.Node.Set(CfgPOWTimeout, 1*time.Minute)
}

func TestPowFilter_Filter(t *testing.T) {
	filter := &powFilter{}

	// set callbacks
	m := &callbackMock{}
	filter.OnAccept(m.Accept)
	filter.OnReject(m.Reject)

	msg := newTestMessage(0).Bytes()

	m.On("Reject", msg, mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrInvalidPOWDifficultly) }), testPeer)
	filter.Filter(msg, testPeer)

	nonce, err := DoPOW(msg)
	require.NoError(t, err)

	msgPOW := newTestMessage(nonce).Bytes()
	require.NoError(t, ValidatePOW(msgPOW))

	m.On("Accept", msgPOW, testPeer)
	filter.Filter(msgPOW, testPeer)

	filter.Shutdown()
	m.AssertExpectations(t)
}

func BenchmarkDoPOW(b *testing.B) {
	messages := make([][]byte, b.N)
	for i := range messages {
		messages[i] = newTestMessage(uint64(i)).Bytes()
	}
	b.ResetTimer()

	for i := range messages {
		_, _ = DoPOW(messages[i])
	}
}

type callbackMock struct{ mock.Mock }

func (m *callbackMock) Accept(msg []byte, p *peer.Peer)            { m.Called(msg, p) }
func (m *callbackMock) Reject(msg []byte, err error, p *peer.Peer) { m.Called(msg, err, p) }

func newTestMessage(nonce uint64) *message.Message {
	return message.New(message.EmptyId, message.EmptyId, time.Time{}, ed25519.PublicKey{}, 0, testPayload, nonce, ed25519.Signature{})
}
