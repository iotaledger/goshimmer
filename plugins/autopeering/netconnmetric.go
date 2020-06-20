package autopeering

import (
	"net"

	"go.uber.org/atomic"
)

// NetConnMetric is a wrapper of a UDPConn that keeps track of RX and TX bytes.
type NetConnMetric struct {
	*net.UDPConn
	rxBytes atomic.Uint64
	txBytes atomic.Uint64
}

// RXBytes returns the RX bytes.
func (nc *NetConnMetric) RXBytes() uint64 {
	return nc.rxBytes.Load()
}

// TXBytes returns the TX bytes.
func (nc *NetConnMetric) TXBytes() uint64 {
	return nc.txBytes.Load()
}

// ReadFromUDP acts like ReadFrom but returns a UDPAddr.
func (nc *NetConnMetric) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	n, addr, err := nc.UDPConn.ReadFromUDP(b)
	nc.rxBytes.Add(uint64(n))
	return n, addr, err
}

// WriteToUDP acts like WriteTo but takes a UDPAddr.
func (nc *NetConnMetric) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	n, err := nc.UDPConn.WriteToUDP(b, addr)
	nc.txBytes.Add(uint64(n))
	return n, err
}
