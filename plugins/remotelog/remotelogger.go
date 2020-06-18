package remotelog

import (
	"encoding/json"
	"fmt"
	"net"
)

// RemoteLoggerConn is a wrapper for a connection to our RemoteLog server.
type RemoteLoggerConn struct {
	conn net.Conn
}

func newRemoteLoggerConn(address string) (*RemoteLoggerConn, error) {
	c, err := net.Dial("udp", address)
	if err != nil {
		return nil, fmt.Errorf("could not create UDP socket to '%s'. %v", address, err)
	}

	return &RemoteLoggerConn{conn: c}, nil
}

// Send sends a message on the RemoteLoggers connection.
func (r *RemoteLoggerConn) Send(msg interface{}) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = r.conn.Write(b)
	if err != nil {
		return err
	}

	return nil
}
