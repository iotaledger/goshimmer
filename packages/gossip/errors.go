package gossip

import "github.com/pkg/errors"

var (
	ErrNotStarted        = errors.New("manager not started")
	ErrClosed            = errors.New("manager closed")
	ErrNotANeighbor      = errors.New("peer is not a neighbor")
	ErrDuplicateNeighbor = errors.New("peer already connected")
	ErrInvalidPacket     = errors.New("invalid packet")
)
