package transport

import (
	"io"
	"testing"

	"github.com/magiconair/properties/assert"
	pb "github.com/wollac/autopeering/proto"
)

var testPacket = &pb.Packet{Data: []byte("TEST")}

func TestReadClosed(t *testing.T) {
	p2p := P2P()
	defer p2p.Close()

	p2p.A.Close()
	_, _, err := p2p.A.ReadFrom()
	assert.Equal(t, err, io.EOF)
}

func TestPacket(t *testing.T) {
	p2p := P2P()
	defer p2p.Close()

	if err := p2p.A.WriteTo(testPacket, p2p.B.LocalAddr()); err != nil {
		t.Error(err)
	}

	pkt, addr, err := p2p.B.ReadFrom()
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, pkt.GetData(), testPacket.GetData())
	assert.Equal(t, addr, p2p.A.LocalAddr())
}
