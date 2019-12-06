package gossip

import "github.com/pkg/errors"

var (
	ErrClosed            = errors.New("manager closed")
	ErrDuplicateNeighbor = errors.New("peer already connected")
)
