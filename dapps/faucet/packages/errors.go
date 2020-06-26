package faucet

import "errors"

var (
	// ErrInvalidAddr represents an error that is triggered when an invalid address is detected.
	ErrInvalidAddr = errors.New("invalid address")
)
