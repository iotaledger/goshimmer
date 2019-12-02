package transport

import (
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChanReadClosed(t *testing.T) {
	network := NewNetwork("A")
	defer network.Close()

	a := network.GetTransport("A")
	a.Close()
	_, _, err := a.ReadFrom()
	assert.EqualError(t, err, io.EOF.Error())
}

func TestChanPacket(t *testing.T) {
	network := NewNetwork("A", "B")
	defer network.Close()

	a := network.GetTransport("A")
	b := network.GetTransport("B")

	err := a.WriteTo(testPacket, b.LocalAddr().String())
	require.NoError(t, err)

	pkt, addr, err := b.ReadFrom()
	require.NoError(t, err)

	assert.Equal(t, pkt, testPacket)
	assert.Equal(t, addr, a.LocalAddr().String())
}

func TestChanConcurrentWrite(t *testing.T) {
	network := NewNetwork("A", "B", "C", "D")
	defer network.Close()

	a := network.GetTransport("A")
	b := network.GetTransport("B")
	c := network.GetTransport("C")
	d := network.GetTransport("D")

	var wg sync.WaitGroup
	numSender := 3

	// reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numSender*1000; i++ {
			_, _, err := d.ReadFrom()
			assert.Equal(t, err, nil)
		}
	}()

	wg.Add(numSender)
	burstWriteTo(a, d.LocalAddr().String(), 1000, &wg)
	burstWriteTo(b, d.LocalAddr().String(), 1000, &wg)
	burstWriteTo(c, d.LocalAddr().String(), 1000, &wg)

	// wait for everything to finish
	wg.Wait()
}

func burstWriteTo(t Transport, addr string, numPackets int, wg *sync.WaitGroup) {
	defer wg.Done()

	go func() {
		for i := 0; i < numPackets; i++ {
			_ = t.WriteTo(testPacket, addr)
		}
	}()
}
