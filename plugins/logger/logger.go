package logger

import (
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/plugins/config"
)

func init() {
	if err := logger.InitGlobalLogger(config.NodeConfig); err != nil {
		panic(err)
	}
}
