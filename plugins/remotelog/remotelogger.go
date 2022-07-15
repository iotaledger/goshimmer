package remotelog

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/core/clock"
	"github.com/iotaledger/goshimmer/plugins/banner"
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

// SendLogBlk sends log block to the remote logger.
func (r *RemoteLoggerConn) SendLogBlk(level logger.Level, name, blk string) {
	m := logBlock{
		banner.AppVersion,
		myGitHead,
		myGitConflict,
		myID,
		level.CapitalString(),
		name,
		blk,
		clock.SyncedTime(),
		remoteLogType,
	}

	_ = deps.RemoteLogger.Send(m)
}

// Send sends a block on the RemoteLoggers connection.
func (r *RemoteLoggerConn) Send(blk interface{}) error {
	b, err := json.Marshal(blk)
	if err != nil {
		return err
	}
	_, err = r.conn.Write(b)
	if err != nil {
		return err
	}

	return nil
}
