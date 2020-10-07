package plugins

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi/data"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/faucet"
	"github.com/iotaledger/goshimmer/plugins/webapi/healthz"
	"github.com/iotaledger/goshimmer/plugins/webapi/info"
	"github.com/iotaledger/goshimmer/plugins/webapi/message"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools"
	"github.com/iotaledger/goshimmer/plugins/webapi/value"
	"github.com/iotaledger/hive.go/node"
)

// WebAPI contains the webapi endpoint plugins of a GoShimmer node.
var WebAPI = node.Plugins(
	webapi.Plugin(),
	data.Plugin(),
	drng.Plugin(),
	faucet.Plugin(),
	healthz.Plugin(),
	message.Plugin(),
	autopeering.Plugin(),
	info.Plugin(),
	value.Plugin(),
	tools.Plugin(),
)
