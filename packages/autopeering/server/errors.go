package server

import "github.com/pkg/errors"

var (
	// ErrTimeout is returned when an expected response was not received in time.
	ErrTimeout = errors.New("response timeout")

	// ErrClosed means that the server was shut down before a response could be received.
	ErrClosed = errors.New("socket closed")

	// ErrNoMessage is returned when the package did not contain any data.
	ErrNoMessage = errors.New("packet does not contain a message")

	// ErrInvalidMessage means that no handler could process the received message.
	ErrInvalidMessage = errors.New("invalid message")
)
