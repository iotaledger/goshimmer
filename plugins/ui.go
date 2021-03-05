package plugins

import (
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/iotaledger/hive.go/node"
)

// UI contains the user interface plugins of a GoShimmer node.
var UI = node.Plugins(
	dashboard.Plugin(),
)
