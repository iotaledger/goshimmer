// Package netutil provides utility functions extending the stdnet package.
package netutil

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"time"
)

var (
	errInvalidData = errors.New("invalid data received")
)

// IsIPv4 returns true if ip is an IPv4 address.
func IsIPv4(ip net.IP) bool {
	return ip.To4() != nil
}

// GetPublicIP queries the ipify API for the public IP address.
func GetPublicIP(preferIPv6 bool) (net.IP, error) {
	var url string
	if preferIPv6 {
		url = "https://api6.ipify.org"
	} else {
		url = "https://api.ipify.org"
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	// the body only consists of the ip address
	ip := net.ParseIP(string(body))
	if ip == nil {
		return nil, fmt.Errorf("not an IP: %s", body)
	}
	return ip, nil
}

// IsTemporaryError checks whether the given error should be considered temporary.
func IsTemporaryError(err error) bool {
	tempErr, ok := err.(interface {
		Temporary() bool
	})
	return ok && tempErr.Temporary()
}

// CheckUDP checks whether data send to remote is received at local, otherwise an error is returned.
// If checkAddress is set, it checks whether the IP address that was on the packet matches remote.
// If checkPort is set, it checks whether the port that was on the packet matches remote.
func CheckUDP(local, remote *net.UDPAddr, checkAddress bool, checkPort bool) error {
	conn, err := net.ListenUDP("udp", local)
	if err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}
	defer conn.Close()

	nonce := generateNonce()
	_, err = conn.WriteTo(nonce, remote)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	err = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		return fmt.Errorf("set timeout failed: %w", err)
	}

	p := make([]byte, len(nonce)+1)
	n, from, err := conn.ReadFrom(p)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}
	if n != len(nonce) || !bytes.Equal(p[:n], nonce) {
		return errInvalidData
	}
	udpAddr := from.(*net.UDPAddr)
	if checkAddress && udpAddr.IP.Equal(remote.IP) {
		return fmt.Errorf("IP changed: %s", udpAddr.IP)
	}
	if checkPort && udpAddr.Port != remote.Port {
		return fmt.Errorf("port changed: %d", udpAddr.Port)
	}

	return nil
}

func generateNonce() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, rand.Uint64())
	return b
}
