package netutil

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsIPv4(t *testing.T) {
	tests := []struct {
		in  net.IP
		out bool
	}{
		{net.IPv4zero, true},
		{net.IPv6zero, false},
		{net.ParseIP("127.0.0.1"), true},
		{net.IPv6loopback, false},
		{net.ParseIP("8.8.8.8"), true},
		{net.ParseIP("2001:4860:4860::8888"), false},
	}
	for _, tt := range tests {
		t.Run(tt.in.String(), func(t *testing.T) {
			assert.Equal(t, IsIPv4(tt.in), tt.out)
		})
	}
}

func TestCheckUDP(t *testing.T) {
	local, err := getLocalUDPAddr()
	require.NoError(t, err)
	assert.NoError(t, CheckUDP(local, local, true, true))

	invalid := &net.UDPAddr{
		IP:   local.IP,
		Port: local.Port - 1,
		Zone: local.Zone,
	}
	assert.Error(t, CheckUDP(local, invalid, false, false))
}

func getLocalUDPAddr() (*net.UDPAddr, error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return conn.LocalAddr().(*net.UDPAddr), conn.Close()
}
