package research

import (
	analysisclient "github.com/iotaledger/goshimmer/plugins/analysis/client"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	remotelog.Plugin,
	analysisserver.Plugin,
	analysisclient.Plugin,
	analysisdashboard.Plugin,
)
