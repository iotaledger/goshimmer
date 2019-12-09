package statusscreen

import (
	"time"

	"github.com/iotaledger/hive.go/logger"
)

func storeStatusMessage(logLevel logger.LogLevel, prefix string, message string) {
	mutex.Lock()
	defer mutex.Unlock()
	messageLog = append(messageLog, &StatusMessage{
		Source:   prefix,
		LogLevel: logLevel,
		Message:  message,
		Time:     time.Now(),
	})

	if statusMessage, exists := statusMessages[prefix]; !exists {
		statusMessages[prefix] = &StatusMessage{
			Source:   prefix,
			LogLevel: logLevel,
			Message:  message,
			Time:     time.Now(),
		}
	} else {
		statusMessage.LogLevel = logLevel
		statusMessage.Message = message
		statusMessage.Time = time.Now()
	}
}
