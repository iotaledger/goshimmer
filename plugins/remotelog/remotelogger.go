package remotelog

import (
	"encoding/json"
	"net"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/logger"
)

// RemoteLoggerConn is a wrapper for a connection to our RemoteLog server.
type RemoteLoggerConn struct {
	conn net.Conn
}

func newRemoteLoggerConn(address string) (*RemoteLoggerConn, error) {
	c, err := net.Dial("udp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create UDP socket to '%s'.", address)
	}

	return &RemoteLoggerConn{conn: c}, nil
}

// SendLogMsg sends log message to the remote logger.
func (r *RemoteLoggerConn) SendLogMsg(level logger.Level, name, msg string) {
	m := logBlock{
		banner.AppVersion,
		myGitHead,
		myGitConflict,
		myID,
		level.CapitalString(),
		name,
		msg,
		time.Now(),
		remoteLogType,
	}

	_ = deps.RemoteLogger.Send(m)
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
