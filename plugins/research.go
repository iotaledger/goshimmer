package plugins

import (
	analysisclient "github.com/iotaledger/goshimmer/plugins/analysis/client"
	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/networkdelay"
	"github.com/iotaledger/goshimmer/plugins/prometheus"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/node"
)

// Research contains research plugins of a GoShimmer node.
var Research = node.Plugins(
	remotelog.Plugin(),
	analysisserver.Plugin(),
	analysisclient.Plugin(),
	analysisdashboard.Plugin(),
	prometheus.Plugin(),
	networkdelay.App(),
)
