package transport

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// Creates a TransportGRPC dialling and listing over the same internal buffer.
func bufGRPC() *TransportGRPC {
	lis := bufconn.Listen(bufSize)
	dial := func(string, time.Duration) (net.Conn, error) {
		// ignore the address and always dial the buffer
		return lis.Dial()
	}

	t := GRPC(lis)
	t.SetDialOptions(grpc.WithInsecure(), grpc.WithDialer(dial))

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
	assert.Equal(t, err, io.EOF)
}

func TestGRPCPacket(t *testing.T) {
	grpc := bufGRPC()
	defer grpc.Close()

	if err := grpc.WriteTo(testPacket, grpc.LocalAddr()); err != nil {
		t.Fatal(err)
	}

	pkt, addr, err := grpc.ReadFrom()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, pkt.GetData(), testPacket.GetData())
	assert.Equal(t, addr, grpc.LocalAddr())
}
