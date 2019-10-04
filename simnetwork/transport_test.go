package simnetwork

import (
	"testing"

	"github.com/stretchr/testify/assert"
	pb "github.com/wollac/autopeering/server/proto"
	"github.com/wollac/autopeering/transport"
)

var testPacket = &pb.Packet{Data: []byte("TEST")}

func TestTransportInterface(t *testing.T) {
	var _ transport.Transport = (*Transport)(nil)
}

func TestCommunication(t *testing.T) {
	peerA := NewTransport("A")
	peerB := NewTransport("B")
	peers := []*Transport{peerA, peerB}

	NewNetwork(peers)

	err := peerA.WriteTo(testPacket, "B")
	assert.Equal(t, err, nil)

	pkt, addr, err := peerB.ReadFrom()
	assert.Equal(t, err, nil)

	assert.Equal(t, pkt, testPacket)
	assert.Equal(t, addr, peerA.LocalAddr())
}

func TestConcurrentCommunication(t *testing.T) {
	peerA := NewTransport("A")
	peerB := NewTransport("B")
	peerC := NewTransport("C")
	peerD := NewTransport("D")
	peers := []*Transport{peerA, peerB, peerC, peerD}

	NewNetwork(peers)

	writeDone := make(chan bool)
	readDone := make(chan bool)
	writerDone := make(chan bool, 3)
	counter := 0

	// reader
	go func() {
		for {
			select {
			case <-writeDone:
				for {
					select {
					case <-peerD.in:
						counter++
					default:
						readDone <- true
						return
					}
				}

			default:
				_, _, _ = peerD.ReadFrom()
				//assert.Equal(t, err, nil)
				counter++

			}
		}
	}()

	peerA.burstSendTo(peerD, 1000, writerDone)
	peerB.burstSendTo(peerD, 1000, writerDone)
	peerC.burstSendTo(peerD, 1000, writerDone)

	for i := 0; i < 3; i++ {
		<-writerDone
	}
	writeDone <- true

	<-readDone

	assert.Equal(t, counter, 3*1000)
}

func (peer *Transport) burstSendTo(dest *Transport, numPkts int, doneSending chan bool) {
	go func() {
		for i := 0; i < numPkts; i++ {
			_ = peer.WriteTo(testPacket, dest.LocalAddr())
		}
		doneSending <- true
	}()
}
