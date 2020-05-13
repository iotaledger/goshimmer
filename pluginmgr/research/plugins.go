package research

import (
	"github.com/iotaledger/goshimmer/dapps/fpctest"
	analysisclient "github.com/iotaledger/goshimmer/plugins/analysis/client"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	analysiswebinterface "github.com/iotaledger/goshimmer/plugins/analysis/webinterface"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	remotelog.Plugin,
	analysisserver.Plugin,
	analysisclient.Plugin,
	analysiswebinterface.Plugin,
	fpctest.App,
)
