package tangle

import (
	"time"
)

const (
	MAX_MISSING_TIME_BEFORE_CLEANUP = 30 * time.Second
	MISSING_CHECK_INTERVAL          = 5 * time.Second
)
