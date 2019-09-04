package transport

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// Creates a GRPC dialing and listing over the same internal buffer.
func bufGRPC() *TransportGRPC {
	lis := bufconn.Listen(bufSize)
	dial := func(context.Context, string) (net.Conn, error) {
		// ignore the address and always dial the buffer
		return lis.Dial()
	}

	t := GRPC(lis)
	t.SetDialOptions(grpc.WithInsecure(), grpc.WithContextDialer(dial))

	return t
}

func TestGRPCStartStop(t *testing.T) {
	grpc := bufGRPC()
	grpc.Close()
}

func TestGRPCReadClosed(t *testing.T) {
	grpc := bufGRPC()

	grpc.Close()
	_, _, err := grpc.ReadFrom()
	assert.EqualError(t, err, io.EOF.Error())
}

func TestGRPCPacket(t *testing.T) {
	grpc := bufGRPC()
	defer grpc.Close()

	err := grpc.WriteTo(testPacket, grpc.LocalAddr())
	require.NoError(t, err)

	pkt, addr, err := grpc.ReadFrom()
	require.NoError(t, err)

	assert.Equal(t, pkt.GetData(), testPacket.GetData())
	assert.Equal(t, addr, grpc.LocalAddr())
}
