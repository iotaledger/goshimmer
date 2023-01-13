package p2p

import "github.com/pkg/errors"

var (
	// ErrNotRunning is returned when a neighbor is added to a stopped or not yet started p2p manager.
	ErrNotRunning = errors.New("manager not running")
	// ErrUnknownNeighbor is returned when the specified neighbor is not known to the p2p manager.
	ErrUnknownNeighbor = errors.New("unknown neighbor")
	// ErrLoopbackNeighbor is returned when the own peer is specified as a neighbor.
	ErrLoopbackNeighbor = errors.New("loopback connection not allowed")
	// ErrDuplicateNeighbor is returned when the same peer is added more than once as a neighbor.
	ErrDuplicateNeighbor = errors.New("already connected")
	// ErrNeighborQueueFull is returned when the send queue is already full.
	ErrNeighborQueueFull = errors.New("send queue is full")
)
