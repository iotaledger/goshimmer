package research

import (
	"github.com/iotaledger/goshimmer/dapps/networkdelay"
	analysisclient "github.com/iotaledger/goshimmer/plugins/analysis/client"
	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/prometheus"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	remotelog.Plugin(),
	analysisserver.Plugin(),
	analysisclient.Plugin(),
	analysisdashboard.Plugin(),
	prometheus.Plugin,
	networkdelay.App(),
)
