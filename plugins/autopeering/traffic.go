package autopeering

import (
	"net"

	"go.uber.org/atomic"
)

// UDPConnTraffic is a wrapper of a UDPConn that keeps track of RX and TX bytes.
type UDPConnTraffic struct {
	*net.UDPConn
	rxBytes atomic.Uint64
	txBytes atomic.Uint64
}

// RXBytes returns the RX bytes.
func (nc *UDPConnTraffic) RXBytes() uint64 {
	return nc.rxBytes.Load()
}

// TXBytes returns the TX bytes.
func (nc *UDPConnTraffic) TXBytes() uint64 {
	return nc.txBytes.Load()
}

// ReadFromUDP acts like ReadFrom but returns a UDPAddr.
func (nc *UDPConnTraffic) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	n, addr, err := nc.UDPConn.ReadFromUDP(b)
	nc.rxBytes.Add(uint64(n))
	return n, addr, err
}

// WriteToUDP acts like WriteTo but takes a UDPAddr.
func (nc *UDPConnTraffic) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	n, err := nc.UDPConn.WriteToUDP(b, addr)
	nc.txBytes.Add(uint64(n))
	return n, err
}
