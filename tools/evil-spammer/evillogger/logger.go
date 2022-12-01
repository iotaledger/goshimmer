package evillogger

import (
	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/logger"
)

var New = logger.NewLogger

func init() {
	config := configuration.New()
	//err := config.Set(logger.ConfigurationKeyOutputPaths, []string{"evil-spammer.log"})
	if err := logger.InitGlobalLogger(config); err != nil {
		panic(err)
	}
	logger.SetLevel(logger.LevelInfo)
}
