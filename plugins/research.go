package plugins

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/activity"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/goshimmer/plugins/remotemetrics"
)

// Research contains research plugins of a GoShimmer node.
var Research = node.Plugins(
	remotelog.Plugin,
	remotemetrics.Plugin,
	activity.Plugin,
)
