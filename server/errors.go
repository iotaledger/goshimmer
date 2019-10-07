package server

import "github.com/pkg/errors"

var (
	ErrTimeout        = errors.New("response timeout")
	ErrClosed         = errors.New("socket closed")
	ErrNoMessage      = errors.New("packet does not contain a message")
	ErrInvalidMessage = errors.New("invalid message")
)
