package simnetwork

import (
	"testing"

	"github.com/magiconair/properties/assert"
	pb "github.com/wollac/autopeering/proto"
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

	done := make(chan bool)
	doneSending := make(chan bool, 3)
	counter := 0

	go func() {
		for {
			select {
			case <-done:
				break
			default:
				_, _, err := peerD.ReadFrom()
				assert.Equal(t, err, nil)
				counter++
			}
		}
	}()

	sendTestBurst(peerA, 1000, doneSending, t)
	sendTestBurst(peerB, 1000, doneSending, t)
	sendTestBurst(peerC, 1000, doneSending, t)

	for i := 0; i < 3; i++ {
		<-doneSending
	}
	done <- true

	assert.Equal(t, counter, 3*1000)
}

func sendTestBurst(peer *Transport, numPkts int, doneSending chan bool, t *testing.T) {
	go func() {
		for i := 0; i < numPkts; i++ {
			err := peer.WriteTo(testPacket, "D")
			assert.Equal(t, err, nil)
		}
		doneSending <- true
	}()
}
