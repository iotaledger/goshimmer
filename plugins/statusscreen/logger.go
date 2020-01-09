package statusscreen

import (
	stdlog "log"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/logger"
)

var (
	mu                sync.Mutex
	logMessages       = make([]*logMessage, 0)
	logMessagesByName = make(map[string]*logMessage)
)

type logMessage struct {
	time  time.Time
	name  string
	level logger.Level
	msg   string
}

func stdLogMsg(level logger.Level, name string, msg string) {
	stdlog.Printf("[ %s ] %s: %s",
		level.CapitalString(),
		name,
		msg,
	)
}

func storeLogMsg(level logger.Level, name string, message string) {
	mu.Lock()
	defer mu.Unlock()

	logMessages = append(logMessages, &logMessage{
		time:  time.Now(),
		name:  name,
		level: level,
		msg:   message,
	})

	if statusMessage, exists := logMessagesByName[name]; !exists {
		logMessagesByName[name] = &logMessage{
			time:  time.Now(),
			name:  name,
			level: level,
			msg:   message,
		}
	} else {
		statusMessage.time = time.Now()
		statusMessage.level = level
		statusMessage.msg = message
	}
}
