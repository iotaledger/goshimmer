package evillogger

import (
	"fmt"

	"github.com/iotaledger/hive.go/app/configuration"
	appLogger "github.com/iotaledger/hive.go/app/logger"
	"github.com/iotaledger/hive.go/logger"
)

var New = logger.NewLogger

func init() {
	config := configuration.New()
	err := config.Set(logger.ConfigurationKeyOutputPaths, []string{"evil-spammer.log", "stdout"})
	if err != nil {
		fmt.Println(err)
		return
	}
	if err = appLogger.InitGlobalLogger(config); err != nil {
		panic(err)
	}
	logger.SetLevel(logger.LevelInfo)
}
