package transport

import (
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/proto"
)

type transport struct {
	in        <-chan transfer
	out       chan<- transfer
	localAddr string

	closeOnce sync.Once
	closing   chan struct{}
}

// transfer represents a send and contains the package and the return address.
type transfer struct {
	pkt  *pb.Packet
	addr string
}

// TransportP2P offers transfers between exactly two clients.
type TransportP2P struct {
	A, B Transport
}

// P2P creates a new in-memory two clients transport network.
// All writes in one client will always be received by the other client, no
// matter what address was specified.
func P2P() *TransportP2P {
	chanA := make(chan transfer, 1)
	chanB := make(chan transfer, 1)

	return &TransportP2P{
		A: new(chanA, chanB, "A"),
		B: new(chanB, chanA, "B"),
	}
}

// Close closes each of the two client transport layers.
func (t *TransportP2P) Close() {
	t.A.Close()
	t.B.Close()
}

func new(in <-chan transfer, out chan<- transfer, address string) *transport {
	return &transport{
		in:        in,
		out:       out,
		localAddr: address,
		closing:   make(chan struct{}),
	}
}

func (t *transport) ReadFrom() (*pb.Packet, string, error) {
	select {
	case res := <-t.in:
		return res.pkt, res.addr, nil
	case <-t.closing:
		return nil, "", io.EOF
	}
}

func (t *transport) WriteTo(pkt *pb.Packet, address string) error {
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

func (t *transport) Close() {
	t.closeOnce.Do(func() {
		close(t.closing)
	})
}

func (t *transport) LocalAddr() string {
	return t.localAddr
}
