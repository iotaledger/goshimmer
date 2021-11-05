package libp2ptesting

import (
	"context"
	"io"
	"testing"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

// NewStreamsPipe returns a pair of libp2p Stream that are talking to each other.
func NewStreamsPipe(t testing.TB) (network.Stream, network.Stream, func()) {
	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)
	host1, host2 := mn.Hosts()[0], mn.Hosts()[1]

	acceptStremCh := make(chan network.Stream, 1)
	host2.SetStreamHandler(protocol.TestingID, func(s network.Stream) {
		b := make([]byte, 4)
		_, err1 := io.ReadFull(s, b)
		require.NoError(t, err1)
		require.Equal(t, b, []byte("beep"))
		_, err1 = s.Write([]byte("boop"))
		require.NoError(t, err1)
		acceptStremCh <- s
	})

	dialStream, err := host1.NewStream(ctx, host2.ID(), protocol.TestingID)
	require.NoError(t, err)
	_, err = dialStream.Write([]byte("beep"))
	require.NoError(t, err)
	b := make([]byte, 4)
	_, err = io.ReadFull(dialStream, b)
	require.NoError(t, err)
	require.Equal(t, b, []byte("boop"))
	acceptStream := <-acceptStremCh
	tearDown := func() {
		err2 := dialStream.Close()
		require.NoError(t, err2)
		err2 = acceptStream.Close()
		require.NoError(t, err2)
		err2 = host1.Close()
		require.NoError(t, err)
		err2 = host2.Close()
		require.NoError(t, err)
	}
	return dialStream, acceptStream, tearDown
}
