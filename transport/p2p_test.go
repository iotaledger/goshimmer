package transport

import (
	"io"
	"net"
	"testing"

	"github.com/magiconair/properties/assert"
	pb "github.com/wollac/autopeering/proto"
)

var testIp = net.ParseIP("127.0.0.1")
var testPacket = &pb.Packet{Data: []byte("TEST")}

func TestReadClosed(t *testing.T) {
	p2p := NewP2P(testIp)
	defer p2p.Close()

	p2p.A.Close()
	_, _, err := p2p.A.Read()
	assert.Equal(t, err, io.EOF)
}

func TestPacket(t *testing.T) {
	p2p := NewP2P(testIp)
	defer p2p.Close()

	if err := p2p.A.Write(testPacket, p2p.B.LocalEndpoint()); err != nil {
		t.Error(err)
	}

	pkt, addr, err := p2p.B.Read()
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, pkt, testPacket)
	assert.Equal(t, addr, p2p.A.LocalEndpoint())
}
