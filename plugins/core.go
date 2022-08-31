package plugins

import (
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/blocklayer"
	"github.com/iotaledger/goshimmer/plugins/bootstrapmanager"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/clock"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/firewall"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/logger"
	"github.com/iotaledger/goshimmer/plugins/manaeventlogger"
	"github.com/iotaledger/goshimmer/plugins/manainitializer"
	"github.com/iotaledger/goshimmer/plugins/manualpeering"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/p2p"
	"github.com/iotaledger/goshimmer/plugins/peer"
	"github.com/iotaledger/goshimmer/plugins/portcheck"
	"github.com/iotaledger/goshimmer/plugins/pow"
	"github.com/iotaledger/goshimmer/plugins/profiling"
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
	database.Plugin,
	peer.Plugin,
	portcheck.Plugin,
	autopeering.Plugin,
	manualpeering.Plugin,
	profiling.Plugin,
	pow.Plugin,
	clock.Plugin,
	blocklayer.Plugin,
	p2p.Plugin,
	gossip.Plugin,
	warpsync.Plugin,
	firewall.Plugin,
	blocklayer.ManaPlugin,
	blocklayer.NotarizationPlugin,
	blocklayer.SnapshotPlugin,
	bootstrapmanager.Plugin,
	faucet.Plugin,
	metrics.Plugin,
	spammer.Plugin,
	manaeventlogger.Plugin,
	manainitializer.Plugin,
)
