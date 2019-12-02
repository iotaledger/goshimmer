package transport

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPacket = []byte("TEST")

func TestP2PReadClosed(t *testing.T) {
	p2p := P2P()
	defer p2p.Close()

	p2p.A.Close()
	_, _, err := p2p.A.ReadFrom()
	assert.EqualError(t, err, io.EOF.Error())
}

func TestP2PPacket(t *testing.T) {
	p2p := P2P()
	defer p2p.Close()

	err := p2p.A.WriteTo(testPacket, p2p.B.LocalAddr().String())
	require.NoError(t, err)

	pkt, addr, err := p2p.B.ReadFrom()
	require.NoError(t, err)

	assert.Equal(t, pkt, testPacket)
	assert.Equal(t, addr, p2p.A.LocalAddr().String())
}
