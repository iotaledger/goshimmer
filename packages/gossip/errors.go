package gossip

import "errors"

var (
	ErrNotStarted        = errors.New("manager not started")
	ErrClosed            = errors.New("manager closed")
	ErrNotANeighbor      = errors.New("peer is not a neighbor")
	ErrLoopback          = errors.New("loopback connection not allowed")
	ErrDuplicateNeighbor = errors.New("peer already connected")
	ErrInvalidPacket     = errors.New("invalid packet")
)
