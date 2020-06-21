package pow

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testPayload               = payload.NewData([]byte("test"))
	testPeer       *peer.Peer = nil
	testDifficulty            = 22
)

func init() {
	// set the test parameters
	config.Node.Set(CfgPOWDifficulty, testDifficulty)
	config.Node.Set(CfgPOWNumThreads, 6)
	config.Node.Set(CfgPOWTimeout, 1*time.Minute)
}

func TestPowFilter_Filter(t *testing.T) {
	filter := &powFilter{}

	// set callbacks
	m := &callbackMock{}
	filter.OnAccept(m.Accept)
	filter.OnReject(m.Reject)

	msg := newTestMessage(0)

	m.On("Reject", msg, ErrInvalidPOWDifficultly, testPeer)
	filter.Filter(msg, testPeer)

	nonce, err := DoPOW(msg)
	require.NoError(t, err)

	msgPOW := newTestMessage(nonce)
	require.NoError(t, ValidatePOW(msgPOW))

	m.On("Accept", msgPOW, testPeer)
	filter.Filter(msgPOW, testPeer)

	filter.Shutdown()
	m.AssertExpectations(t)
}

func BenchmarkDoPOW(b *testing.B) {
	messages := make([]*message.Message, b.N)
	for i := range messages {
		messages[i] = newTestMessage(uint64(i))
	}
	b.ResetTimer()

	for i := range messages {
		_, _ = DoPOW(messages[i])
	}
}

type callbackMock struct{ mock.Mock }

func (m *callbackMock) Accept(msg *message.Message, p *peer.Peer)            { m.Called(msg, p) }
func (m *callbackMock) Reject(msg *message.Message, err error, p *peer.Peer) { m.Called(msg, err, p) }

func newTestMessage(nonce uint64) *message.Message {
	return message.New(message.EmptyId, message.EmptyId, time.Time{}, ed25519.PublicKey{}, 0, testPayload, nonce, ed25519.Signature{})
}
