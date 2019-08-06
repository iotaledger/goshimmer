package statusscreen

import (
	"time"
)

type StatusMessage struct {
	Source   string
	LogLevel int
	Message  string
	Time     time.Time
}
