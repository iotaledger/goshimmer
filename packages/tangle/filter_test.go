package tangle

import (
	"context"
	"crypto"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b" // required by crypto.BLAKE2b_512
)

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

func TestAPowFilter_Filter(t *testing.T) {
	filter := NewPowFilter(testWorker, testDifficulty)
	pow.BaseDifficulty = testDifficulty
	pow.ApowWindow = testWindow
	pow.AdaptiveRate = testRate

	// set callbacks
	m := &callbackMock{}
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
	m := &callbackMock{}
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

type callbackMock struct{ mock.Mock }

func (m *callbackMock) Accept(msg *Message, p *peer.Peer)            { m.Called(msg, p) }
func (m *callbackMock) Reject(msg *Message, err error, p *peer.Peer) { m.Called(msg, err, p) }
