package plugins

import (
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi/data"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/faucet"
	"github.com/iotaledger/goshimmer/plugins/webapi/healthz"
	"github.com/iotaledger/goshimmer/plugins/webapi/info"
	"github.com/iotaledger/goshimmer/plugins/webapi/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/webapi/mana"
	"github.com/iotaledger/goshimmer/plugins/webapi/message"
	"github.com/iotaledger/goshimmer/plugins/webapi/snapshot"
	drngTools "github.com/iotaledger/goshimmer/plugins/webapi/tools/drng"
	msgTools "github.com/iotaledger/goshimmer/plugins/webapi/tools/message"
	"github.com/iotaledger/goshimmer/plugins/webapi/weightprovider"
)

// WebAPI contains the webapi endpoint plugins of a GoShimmer node.
var WebAPI = node.Plugins(
	webapi.Plugin,
	data.Plugin,
	drng.Plugin,
	faucet.Plugin,
	healthz.Plugin,
	message.Plugin,
	autopeering.Plugin,
	info.Plugin,
	drngTools.Plugin,
	msgTools.Plugin,
	mana.Plugin,
	ledgerstate.Plugin,
	snapshot.Plugin,
	weightprovider.Plugin,
)
