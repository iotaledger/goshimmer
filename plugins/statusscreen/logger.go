package statusscreen

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/node"
)

func storeStatusMessage(pluginName string, message string, logLevel int) {
	messageLog = append(messageLog, &StatusMessage{
		Source:   pluginName,
		LogLevel: logLevel,
		Message:  message,
		Time:     time.Now(),
	})

	if statusMessage, exists := statusMessages[pluginName]; !exists {
		statusMessages[pluginName] = &StatusMessage{
			Source:   pluginName,
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

var DEFAULT_LOGGER = &node.Logger{
	Enabled: true,
	LogInfo: func(pluginName string, message string) {
		storeStatusMessage(pluginName, message, node.LOG_LEVEL_INFO)
	},
	LogSuccess: func(pluginName string, message string) {
		storeStatusMessage(pluginName, message, node.LOG_LEVEL_SUCCESS)
	},
	LogWarning: func(pluginName string, message string) {
		storeStatusMessage(pluginName, message, node.LOG_LEVEL_WARNING)
	},
	LogFailure: func(pluginName string, message string) {
		storeStatusMessage(pluginName, message, node.LOG_LEVEL_FAILURE)
	},
	LogDebug: func(pluginName string, message string) {
		storeStatusMessage(pluginName, message, node.LOG_LEVEL_DEBUG)
	},
}
