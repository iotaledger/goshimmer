package plugins

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/clock"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/consensus"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/logger"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/portcheck"
	"github.com/iotaledger/goshimmer/plugins/pow"
	"github.com/iotaledger/goshimmer/plugins/profiling"
	"github.com/iotaledger/goshimmer/plugins/spammer"
	"github.com/iotaledger/goshimmer/plugins/syncbeacon"
	"github.com/iotaledger/goshimmer/plugins/syncbeaconfollower"
	"github.com/iotaledger/hive.go/node"
)

// Core contains the core plugins of a GoShimmer node.
var Core = node.Plugins(
	banner.Plugin(),
	config.Plugin(),
	logger.Plugin(),
	cli.Plugin(),
	gracefulshutdown.Plugin(),
	portcheck.Plugin(),
	profiling.Plugin(),
	database.Plugin(),
	autopeering.Plugin(),
	pow.Plugin(),
	clock.Plugin(),
	messagelayer.Plugin(),
	gossip.Plugin(),
	issuer.Plugin(),
	syncbeacon.Plugin(),
	syncbeaconfollower.Plugin(),
	drng.Plugin(),
	faucet.Plugin(),
	valuetransfers.App(),
	consensus.Plugin(),
	metrics.Plugin(),
	spammer.Plugin(),
)
