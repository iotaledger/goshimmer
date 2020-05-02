package bootstrap

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/spammer"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/sync"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the plugin name of the bootstrap plugin.
	PluginName = "Bootstrap"
	// CfgBootstrapInitialIssuanceTimePeriodSec defines the initial time period of how long the node should be
	// issuing messages when started in bootstrapping mode. If the value is set to -1, the issuance is continuous.
	CfgBootstrapInitialIssuanceTimePeriodSec = "bootstrap.initialIssuance.timePeriodSec"
	// the messages per second to issue when in bootstrapping mode.
	initialIssuanceMPS = 1
	// the value which determines a continuous issuance of messages from the bootstrap plugin.
	continuousIssuance = -1
)

func init() {
	flag.Int(CfgBootstrapInitialIssuanceTimePeriodSec, -1, "the initial time period of how long the node should be issuing messages when started in bootstrapping mode. "+
		"if the value is set to -1, the issuance is continuous.")
}

var (
	// Plugin is the plugin instance of the bootstrap plugin.
	Plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	log    *logger.Logger
)

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	// we're auto. synced if we start in bootstrapping mode
	sync.OverwriteSyncedState(true)
	log.Infof("starting node in bootstrapping mode")
}

func run(_ *node.Plugin) {

	messageSpammer := spammer.New(issuer.IssuePayload)
	issuancePeriodSec := config.Node.GetInt(CfgBootstrapInitialIssuanceTimePeriodSec)
	issuancePeriod := time.Duration(issuancePeriodSec) * time.Second

	// issue messages on top of the genesis
	_ = daemon.BackgroundWorker("Bootstrapping-Issuer", func(shutdownSignal <-chan struct{}) {
		messageSpammer.Start(initialIssuanceMPS)
		defer messageSpammer.Shutdown()
		// don't stop issuing messages if in continuous mode
		if issuancePeriodSec == continuousIssuance {
			log.Info("continuously issuing bootstrapping messages")
			<-shutdownSignal
			return
		}
		log.Infof("issuing bootstrapping messages for %d seconds", issuancePeriodSec)
		select {
		case <-time.After(issuancePeriod):
		case <-shutdownSignal:
		}
	}, shutdown.PriorityBootstrap)
}
