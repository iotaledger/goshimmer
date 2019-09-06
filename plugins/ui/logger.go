package ui

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/node"
)

var logHistory = make([]*statusMessage, 0)

type statusMessage struct {
	Source  string    `json:"source"`
	Level   int       `json:"level"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

type resp map[string]interface{}

func storeAndSendStatusMessage(pluginName string, message string, level int) {

	msg := &statusMessage{
		Source:  pluginName,
		Level:   level,
		Message: message,
		Time:    time.Now(),
	}
	logHistory = append(logHistory, msg)
	ws.send(resp{
		"logs": []*statusMessage{msg},
	})
}

var uiLogger = &node.Logger{
	LogInfo: func(pluginName string, message string) {
		storeAndSendStatusMessage(pluginName, message, node.LOG_LEVEL_INFO)
	},
	LogSuccess: func(pluginName string, message string) {
		storeAndSendStatusMessage(pluginName, message, node.LOG_LEVEL_SUCCESS)
	},
	LogWarning: func(pluginName string, message string) {
		storeAndSendStatusMessage(pluginName, message, node.LOG_LEVEL_WARNING)
	},
	LogFailure: func(pluginName string, message string) {
		storeAndSendStatusMessage(pluginName, message, node.LOG_LEVEL_FAILURE)
	},
	LogDebug: func(pluginName string, message string) {
		storeAndSendStatusMessage(pluginName, message, node.LOG_LEVEL_DEBUG)
	},
}
