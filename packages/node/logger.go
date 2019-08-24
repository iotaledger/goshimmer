package node

import (
	"fmt"
	"sync"
)

type Logger struct {
	enabled      bool
	enabledMutex sync.RWMutex
	LogInfo      func(pluginName string, message string)
	LogSuccess   func(pluginName string, message string)
	LogWarning   func(pluginName string, message string)
	LogFailure   func(pluginName string, message string)
	LogDebug     func(pluginName string, message string)
}

func (logger *Logger) SetEnabled(value bool) {
	logger.enabledMutex.Lock()
	logger.enabled = value
	logger.enabledMutex.Unlock()
}

func (logger *Logger) GetEnabled() (result bool) {
	logger.enabledMutex.RLock()
	result = logger.enabled
	logger.enabledMutex.RUnlock()
	return
}

func pluginPrefix(pluginName string) string {
	var pluginPrefix string
	if pluginName == "Node" {
		pluginPrefix = ""
	} else {
		pluginPrefix = pluginName + ": "
	}

	return pluginPrefix
}

var DEFAULT_LOGGER = &Logger{
	enabled: true,
	LogSuccess: func(pluginName string, message string) {
		fmt.Println("[  OK  ] " + pluginPrefix(pluginName) + message)
	},
	LogInfo: func(pluginName string, message string) {
		fmt.Println("[ INFO ] " + pluginPrefix(pluginName) + message)
	},
	LogWarning: func(pluginName string, message string) {
		fmt.Println("[ WARN ] " + pluginPrefix(pluginName) + message)
	},
	LogFailure: func(pluginName string, message string) {
		fmt.Println("[ FAIL ] " + pluginPrefix(pluginName) + message)
	},
	LogDebug: func(pluginName string, message string) {
		fmt.Println("[ NOTE ] " + pluginPrefix(pluginName) + message)
	},
}
