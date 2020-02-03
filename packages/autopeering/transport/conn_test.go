package transport

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnUdpClosed(t *testing.T) {
	conn := openUDP(t)

	conn.Close()
	_, _, err := conn.ReadFrom()
	assert.EqualError(t, err, io.EOF.Error())
}

func TestConnUdpPacket(t *testing.T) {
	a := openUDP(t)
	defer a.Close()
	b := openUDP(t)
	defer b.Close()

	err := a.WriteTo(testPacket, b.LocalAddr().String())
	require.NoError(t, err)

	pkt, addr, err := b.ReadFrom()
	require.NoError(t, err)

	assert.Equal(t, pkt, testPacket)
	assert.Equal(t, addr, a.LocalAddr().String())
}

func openUDP(t *testing.T) *TransportConn {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	return Conn(c, func(network, address string) (net.Addr, error) { return net.ResolveUDPAddr(network, address) })
}
