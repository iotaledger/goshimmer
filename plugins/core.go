package plugins

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/blockissuer"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/dashboardmetrics"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/indexer"
	"github.com/iotaledger/goshimmer/plugins/logger"
	"github.com/iotaledger/goshimmer/plugins/manainitializer"
	"github.com/iotaledger/goshimmer/plugins/manualpeering"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/p2p"
	"github.com/iotaledger/goshimmer/plugins/peer"
	"github.com/iotaledger/goshimmer/plugins/portcheck"
	"github.com/iotaledger/goshimmer/plugins/profiling"
	"github.com/iotaledger/goshimmer/plugins/profilingrecorder"
	"github.com/iotaledger/goshimmer/plugins/protocol"
	"github.com/iotaledger/goshimmer/plugins/retainer"
	"github.com/iotaledger/goshimmer/plugins/spammer"
	"github.com/iotaledger/goshimmer/plugins/warpsync"
)

// Core contains the core plugins of a GoShimmer node.
var Core = node.Plugins(
	banner.Plugin,
	config.Plugin,
	logger.Plugin,
	cli.Plugin,
	gracefulshutdown.Plugin,
	peer.Plugin,
	portcheck.Plugin,
	autopeering.Plugin,
	manualpeering.Plugin,
	profiling.Plugin,
	profilingrecorder.Plugin,
	p2p.Plugin,
	protocol.Plugin,
	retainer.Plugin,
	indexer.Plugin,
	warpsync.Plugin,
	faucet.Plugin,
	dashboardmetrics.Plugin,
	metrics.Plugin,
	spammer.Plugin,
	manainitializer.Plugin,
	blockissuer.Plugin,
)
