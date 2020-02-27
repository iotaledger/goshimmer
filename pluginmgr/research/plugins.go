package research

import (
	"github.com/iotaledger/goshimmer/plugins/analysis"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	remotelog.PLUGIN,
	analysis.PLUGIN,
	metrics.PLUGIN,
)
