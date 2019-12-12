package ui

import (
	"github.com/iotaledger/hive.go/logger"
	"sync"
	"time"
)

var logMutex = sync.Mutex{}
var logHistory = make([]*statusMessage, 0)

type statusMessage struct {
	Source  string          `json:"source"`
	Level   logger.LogLevel `json:"level"`
	Message string          `json:"message"`
	Time    time.Time       `json:"time"`
}

type resp map[string]interface{}

func storeAndSendStatusMessage(logLevel logger.LogLevel, pluginName string, message string) {

	msg := &statusMessage{
		Source:  pluginName,
		Level:   logLevel,
		Message: message,
		Time:    time.Now(),
	}
	logMutex.Lock()
	logHistory = append(logHistory, msg)
	logMutex.Unlock()
	ws.send(resp{"logs": []*statusMessage{msg}})
}
