package simnetwork

import (
	"errors"
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/server/proto"
	"github.com/wollac/autopeering/transport"
)

type Transport struct {
	transport.Transport // must implement the generic Transport interface
	localAddr           string
	in                  chan transfer

	closeOnce sync.Once
	closing   chan struct{}
}

// transfer represents a send and contains the package and the return address.
type transfer struct {
	pkt  *pb.Packet
	addr string
}

func NewTransport(addr string) *Transport {
	return &Transport{
		localAddr: addr,
		in:        make(chan transfer, QueueSize),
		closing:   make(chan struct{}, 1),
	}
}

func (t *Transport) Close() {
	t.closeOnce.Do(func() {
		close(t.closing)
		close(t.in)
	})
}

func (t *Transport) ReadFrom() (*pb.Packet, string, error) {
	select {
	case res := <-t.in:
		return res.pkt, res.addr, nil
	case <-t.closing:
		return nil, "", io.EOF
	}
}

func (t *Transport) WriteTo(pkt *pb.Packet, to string) error {
	if _, online := Network[to]; !online {
		return errors.New("could not determine peer")
	}

	// clone the packet before sending, just to make sure...
	req := transfer{pkt: &pb.Packet{}, addr: t.localAddr}
	proto.Merge(req.pkt, pkt)

	select {
	case Network[to].in <- req:
		return nil
	case <-t.closing:
		return errors.New("could not determine peer")
	}
}

func (t *Transport) LocalAddr() string {
	return t.localAddr
}

// func (t *Transport) Listen() {
// 	go func() {
// 		for {
// 			select {
// 			case <-t.closing:
// 				break
// 			default:
// 				_, _, err := t.ReadFrom()
// 				if err != nil {
// 					//handle error
// 				} else {
// 					//handle packet
// 				}
// 			}
// 		}
// 	}()
// }
