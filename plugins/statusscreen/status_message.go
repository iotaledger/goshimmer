package statusscreen

import (
	"time"

	"github.com/iotaledger/hive.go/logger"
)

type StatusMessage struct {
	Source   string
	LogLevel logger.Level
	Message  string
	Time     time.Time
}
