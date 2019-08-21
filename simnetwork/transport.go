package simnetwork

import (
	pb "github.com/wollac/autopeering/proto"
)

type Transport struct {
	addr string

	ch chan *pb.Packet
}

func NewTransport(addr string) *Transport {
	return &Transport{
		addr: addr,
		ch:   make(chan *pb.Packet, QueueSize),
	}
}

func (t *Transport) Close() {
	close(t.ch)
}

func (t *Transport) Read() (*pb.Packet, string, error) {
	res := <-t.ch
	return res, t.addr, nil
}

func (t *Transport) Write(req *pb.Packet, to string) error {
	Network[to].ch <- req
	return nil
}

func (t *Transport) LocalEndpoint() string {
	return t.addr
}
