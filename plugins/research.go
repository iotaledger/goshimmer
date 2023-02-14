package plugins

import (
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/plugins/activity"
	analysisclient "github.com/iotaledger/goshimmer/plugins/analysis/client"
	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/networkdelay"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/goshimmer/plugins/remotemetrics"
)

// Research contains research plugins of a GoShimmer node.
var Research = node.Plugins(
	remotelog.Plugin,
	analysisserver.Plugin,
	analysisclient.Plugin,
	analysisdashboard.Plugin,
	remotemetrics.Plugin,
	networkdelay.App(),
	activity.Plugin,
)
