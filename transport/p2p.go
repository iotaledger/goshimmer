package transport

import (
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/server/proto"
)

// TransportP2P offers transfers between exactly two clients.
type TransportP2P struct {
	A, B Transport
}

// P2P creates a new in-memory two clients transport network.
// All writes in one client will always be received by the other client, no
// matter what address was specified.
func P2P() *TransportP2P {
	chanA := make(chan transfer, 5)
	chanB := make(chan transfer, 5)

	return &TransportP2P{
		A: newChanTransport(chanA, chanB, "A"),
		B: newChanTransport(chanB, chanA, "B"),
	}
}

// Close closes each of the two client transport layers.
func (t *TransportP2P) Close() {
	t.A.Close()
	t.B.Close()
}

// chanTransport implements Transport by reading and writing to given channels.
type chanTransport struct {
	in        <-chan transfer
	out       chan<- transfer
	localAddr string

	closeOnce sync.Once
	closing   chan struct{}
}

func newChanTransport(in <-chan transfer, out chan<- transfer, address string) *chanTransport {
	return &chanTransport{
		in:        in,
		out:       out,
		localAddr: address,
		closing:   make(chan struct{}),
	}
}

// ReadFrom implements the Transport ReadFrom method.
func (t *chanTransport) ReadFrom() (*pb.Packet, string, error) {
	select {
	case res := <-t.in:
		return res.pkt, res.addr, nil
	case <-t.closing:
		return nil, "", io.EOF
	}
}

// WriteTo implements the Transport WriteTo method.
func (t *chanTransport) WriteTo(pkt *pb.Packet, address string) error {
	// clone the packet before sending, just to make sure...
	req := transfer{pkt: &pb.Packet{}, addr: t.localAddr}
	proto.Merge(req.pkt, pkt)

	select {
	case t.out <- req:
		return nil
	case <-t.closing:
		return errClosed
	}
}

// Close closes the transport layer.
func (t *chanTransport) Close() {
	t.closeOnce.Do(func() {
		close(t.closing)
	})
}

// LocalAddr returns the local network address.
func (t *chanTransport) LocalAddr() string {
	return t.localAddr
}
