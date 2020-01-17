package portchecker

import (
	"net"
	"time"
)

// OpenTCP opens a TCP port and checks that is reachable
func OpenTCP(addr string) (err error) {
	timeout := time.Second

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		return err
	}
	defer ln.Close()
	go func() {
		for {
			ln.Accept()
		}
	}()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	return err
}
