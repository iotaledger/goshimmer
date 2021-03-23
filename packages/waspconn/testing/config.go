package testing

import (
	"github.com/iotaledger/goshimmer/packages/waspconn"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var log *logger.Logger

func Config(_ *node.Plugin, setLog *logger.Logger, vtangle waspconn.Ledger) {
	log = setLog.Named("testing")
	addEndpoints(vtangle)
}
