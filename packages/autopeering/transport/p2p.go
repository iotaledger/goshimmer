package transport

import (
	"io"
	"net"
	"sync"
)

// TransportP2P offers transfers between exactly two clients.
type TransportP2P struct {
	A, B Transport
}

// P2P creates a new in-memory two clients transport network.
// All writes in one client will always be received by the other client, no
// matter what address was specified.
func P2P() *TransportP2P {
	chanA := make(chan transfer, queueSize)
	chanB := make(chan transfer, queueSize)

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

// p2pAddr represents the address of an p2p end point.
type p2pAddr struct {
	address string
}

func (a p2pAddr) Network() string { return "p2p" }
func (a p2pAddr) String() string  { return a.address }

// chanTransport implements Transport by reading and writing to given channels.
type chanTransport struct {
	in   <-chan transfer
	out  chan<- transfer
	addr p2pAddr

	closeOnce sync.Once
	closing   chan struct{}
}

func newChanTransport(in <-chan transfer, out chan<- transfer, address string) *chanTransport {
	return &chanTransport{
		in:      in,
		out:     out,
		addr:    p2pAddr{address: address},
		closing: make(chan struct{}),
	}
}

// ReadFrom implements the Transport ReadFrom method.
func (t *chanTransport) ReadFrom() ([]byte, string, error) {
	select {
	case res := <-t.in:
		return res.pkt, res.addr, nil
	case <-t.closing:
		return nil, "", io.EOF
	}
}

// WriteTo implements the Transport WriteTo method.
func (t *chanTransport) WriteTo(pkt []byte, _ string) error {
	// clone the packet before sending, just to make sure...
	req := transfer{pkt: append([]byte{}, pkt...), addr: t.addr.address}

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
func (t *chanTransport) LocalAddr() net.Addr {
	return t.addr
}
