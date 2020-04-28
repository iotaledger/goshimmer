package gossip

import "errors"

var (
	ErrNotRunning        = errors.New("manager not running")
	ErrNotANeighbor      = errors.New("peer is not a neighbor")
	ErrLoopback          = errors.New("loopback connection not allowed")
	ErrDuplicateNeighbor = errors.New("peer already connected")
	ErrInvalidPacket     = errors.New("invalid packet")
)
