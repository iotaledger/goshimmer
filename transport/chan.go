package transport

import (
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	pb "github.com/iotaledger/autopeering-sim/server/proto"
)

// ChanNetwork offers in-memory transfers between an arbirtrary number of clients.
type ChanNetwork struct {
	peers map[string]*chanPeer
}

type chanPeer struct {
	network   *ChanNetwork
	localAddr string

	c         chan transfer
	closeOnce sync.Once
	closing   chan struct{}
}

// NewNetwork creates a new in-memory transport network.
// For each provided address a coresponding client is created.
func NewNetwork(addrs ...string) *ChanNetwork {
	network := &ChanNetwork{
		peers: make(map[string]*chanPeer, len(addrs)),
	}

	for _, addr := range addrs {
		network.AddTransport(addr)
	}

	return network
}

// AddTransport adds a new client transport layer to the network.
func (n *ChanNetwork) AddTransport(addr string) {
	n.peers[addr] = newChanPeer(addr, n)
}

// GetTransport returns the corresponding client transport layer for the provided address.
// This function will panic, if no transport layer for that address exists.
func (n *ChanNetwork) GetTransport(addr string) Transport {
	peer, ok := n.peers[addr]
	if !ok {
		panic(errPeer.Error())
	}

	return peer
}

// Close closes each of the peers' transport layers.
func (n *ChanNetwork) Close() {
	for _, peer := range n.peers {
		peer.Close()
	}
}

func newChanPeer(addr string, network *ChanNetwork) *chanPeer {
	return &chanPeer{
		localAddr: addr,
		network:   network,
		c:         make(chan transfer, queueSize),
		closing:   make(chan struct{}),
	}
}

// ReadFrom implements the Transport ReadFrom method.
func (p *chanPeer) ReadFrom() (*pb.Packet, string, error) {
	select {
	case res := <-p.c:
		return res.pkt, res.addr, nil
	case <-p.closing:
		return nil, "", io.EOF
	}
}

// WriteTo implements the Transport WriteTo method.
func (p *chanPeer) WriteTo(pkt *pb.Packet, address string) error {
	// determine the receiving peer
	peer, ok := p.network.peers[address]
	if !ok {
		return errPeer
	}

	// clone the packet before sending, just to make sure...
	req := transfer{pkt: &pb.Packet{}, addr: p.localAddr}
	proto.Merge(req.pkt, pkt)

	select {
	case peer.c <- req:
		return nil
	case <-p.closing:
		return errClosed
	}
}

// Close closes the transport layer.
func (p *chanPeer) Close() {
	p.closeOnce.Do(func() {
		close(p.closing)
	})
}

// LocalAddr returns the local network address.
func (p *chanPeer) LocalAddr() string {
	return p.localAddr
}
