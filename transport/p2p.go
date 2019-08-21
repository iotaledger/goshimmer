package transport

import (
	"errors"
	"io"
	"net"
	"sync"

	pb "github.com/wollac/autopeering/proto"
)

// Errors
var (
	errClosed = errors.New("socket closed")
)

type transport struct {
	in     <-chan request
	out    chan<- request
	local  *Addr
	closed bool

	closeOnce sync.Once
	closing   chan struct{}
}

type request struct {
	pkt  *pb.Packet
	addr *Addr
}

type TransportP2P struct {
	A, B Transport
}

func NewP2P(ip net.IP) *TransportP2P {
	chanA := make(chan request, 1)
	chanB := make(chan request, 1)

	return &TransportP2P{
		A: new(chanA, chanB, &Addr{IP: ip, Port: 1}),
		B: new(chanB, chanA, &Addr{IP: ip, Port: 2}),
	}
}

func (t *TransportP2P) Close() {
	t.A.Close()
	t.B.Close()
}

func new(in <-chan request, out chan<- request, local *Addr) *transport {
	return &transport{
		in:      in,
		out:     out,
		local:   local,
		closing: make(chan struct{}),
	}
}

func (t *transport) Read() (*pb.Packet, *Addr, error) {
	select {
	case req := <-t.in:
		return req.pkt, req.addr, nil
	case <-t.closing:
		return nil, nil, io.EOF
	}
}

func (t *transport) Write(pkt *pb.Packet, addr *Addr) error {
	select {
	case t.out <- request{pkt, t.local}:
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

func (t *transport) LocalEndpoint() *Addr {
	return t.local
}
