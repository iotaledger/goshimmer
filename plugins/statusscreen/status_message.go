package statusscreen

import (
	"time"

	"github.com/iotaledger/hive.go/logger"
)

type StatusMessage struct {
	Source   string
	LogLevel logger.LogLevel
	Message  string
	Time     time.Time
}
