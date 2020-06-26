package core

import (
	"github.com/iotaledger/goshimmer/dapps/faucet"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/bootstrap"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/logger"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/portcheck"
	"github.com/iotaledger/goshimmer/plugins/pow"
	"github.com/iotaledger/goshimmer/plugins/profiling"
	"github.com/iotaledger/goshimmer/plugins/sync"

	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	banner.Plugin(),
	config.Plugin(),
	logger.Plugin(),
	cli.Plugin(),
	gracefulshutdown.Plugin(),
	portcheck.Plugin(),
	profiling.Plugin(),
	database.Plugin(),
	autopeering.Plugin(),
	pow.Plugin,
	messagelayer.Plugin(),
	gossip.Plugin(),
	issuer.Plugin(),
	bootstrap.Plugin(),
	sync.Plugin(),
	metrics.Plugin(),
	drng.Plugin(),
	faucet.App(),
	valuetransfers.App(),
)
