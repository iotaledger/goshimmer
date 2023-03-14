package plugins

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi/block"
	"github.com/iotaledger/goshimmer/plugins/webapi/data"
	"github.com/iotaledger/goshimmer/plugins/webapi/faucet"
	"github.com/iotaledger/goshimmer/plugins/webapi/faucetrequest"
	"github.com/iotaledger/goshimmer/plugins/webapi/healthz"
	"github.com/iotaledger/goshimmer/plugins/webapi/info"
	"github.com/iotaledger/goshimmer/plugins/webapi/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/webapi/mana"
	"github.com/iotaledger/goshimmer/plugins/webapi/ratesetter"
	"github.com/iotaledger/goshimmer/plugins/webapi/scheduler"
	"github.com/iotaledger/goshimmer/plugins/webapi/slot"
	"github.com/iotaledger/goshimmer/plugins/webapi/snapshot"
	"github.com/iotaledger/goshimmer/plugins/webapi/weightprovider"
)

// WebAPI contains the webapi endpoint plugins of a GoShimmer node.
var WebAPI = node.Plugins(
	webapi.Plugin,
	data.Plugin,
	faucetrequest.Plugin,
	faucet.Plugin,
	healthz.Plugin,
	block.Plugin,
	autopeering.Plugin,
	info.Plugin,
	slot.Plugin,
	mana.Plugin,
	ledgerstate.Plugin,
	snapshot.Plugin,
	weightprovider.Plugin,
	ratesetter.Plugin,
	scheduler.Plugin,
)
