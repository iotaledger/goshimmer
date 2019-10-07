package neighborhood

import (
	"time"

	"go.uber.org/zap"
)

// Config holds settings for the peer selection.
type Config struct {
	// Logger
	Log *zap.SugaredLogger

	// Lifetime of the local private salt
	SaltLifetime time.Duration
}
