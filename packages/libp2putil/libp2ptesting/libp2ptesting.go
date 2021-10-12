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

func NewStreamsPipe(t testing.TB) (network.Stream, network.Stream, func()) {
	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 2)
	host1, host2 := mn.Hosts()[0], mn.Hosts()[1]

	acceptStremCh := make(chan network.Stream, 1)
	host2.SetStreamHandler(protocol.TestingID, func(s network.Stream) {
		b := make([]byte, 4)
		_, err := io.ReadFull(s, b)
		require.NoError(t, err)
		require.Equal(t, b, []byte("beep"))
		_, err = s.Write([]byte("boop"))
		require.NoError(t, err)
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
		err := dialStream.Close()
		require.NoError(t, err)
		err = acceptStream.Close()
		require.NoError(t, err)
	}
	return dialStream, acceptStream, tearDown
}
