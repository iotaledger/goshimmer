package gossip

import "github.com/pkg/errors"

var (
	ErrClosed            = errors.New("manager closed")
	ErrNotANeighbor      = errors.New("peer is not a neighbor")
	ErrDuplicateNeighbor = errors.New("peer already connected")
)
