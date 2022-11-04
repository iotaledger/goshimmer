package libp2ptesting

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/stretchr/testify/require"
)

// NewStreamsPipe returns a pair of libp2p Stream that are talking to each other.
func NewStreamsPipe(t testing.TB) (network.Stream, network.Stream, func()) {
	ctx := context.Background()
	host1, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.DisableRelay(),
	)
	require.NoError(t, err)
	host2, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.DisableRelay(),
	)
	require.NoError(t, err)
	acceptStremCh := make(chan network.Stream, 1)
	host2.Peerstore().AddAddrs(host1.ID(), host1.Addrs(), peerstore.PermanentAddrTTL)
	host2.SetStreamHandler(protocol.TestingID, func(s network.Stream) {
		acceptStremCh <- s
	})
	host1.Peerstore().AddAddrs(host2.ID(), host2.Addrs(), peerstore.PermanentAddrTTL)
	dialStream, err := host1.NewStream(ctx, host2.ID(), protocol.TestingID)
	require.NoError(t, err)
	_, err = dialStream.Write(nil)
	require.NoError(t, err)
	acceptStream := <-acceptStremCh
	tearDown := func() {
		err2 := dialStream.Close()
		require.NoError(t, err2)
		err2 = acceptStream.Close()
		require.NoError(t, err2)
		err2 = host1.Close()
		require.NoError(t, err2)
		err2 = host2.Close()
		require.NoError(t, err2)
	}
	return dialStream, acceptStream, tearDown
}
