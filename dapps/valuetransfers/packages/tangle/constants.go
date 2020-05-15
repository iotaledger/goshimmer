package tangle

import (
	"time"
)

const (
	// MaxMissingTimeBeforeCleanup defines how long a transaction can be "missing", before we start pruning its future
	// cone.
	MaxMissingTimeBeforeCleanup = 30 * time.Second

	// MissingCheckInterval defines how often we check if missing transactions have been received.
	MissingCheckInterval = 5 * time.Second
)
