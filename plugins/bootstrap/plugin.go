package bootstrap

import (
	goSync "sync"
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
	// CfgBootstrapTimeUnit defines the time unit (in seconds) of the issuance rate (e.g., 3 messages per 12 seconds).
	CfgBootstrapTimeUnit = "bootstrap.timeUnit"
	// the messages per period to issue when in bootstrapping mode.
	initialIssuanceRate = 1
	// the value which determines a continuous issuance of messages from the bootstrap plugin.
	continuousIssuance = -1
)

func init() {
	flag.Int(CfgBootstrapInitialIssuanceTimePeriodSec, -1, "the initial time period of how long the node should be issuing messages when started in bootstrapping mode. "+
		"if the value is set to -1, the issuance is continuous.")
	flag.Int(CfgBootstrapTimeUnit, 5, "the time unit (in seconds) of the issuance rate (e.g., 1 messages per 5 seconds).")
}

var (
	// plugin is the plugin instance of the bootstrap plugin.
	plugin *node.Plugin
	once   goSync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	// we're auto. synced if we start in bootstrapping mode
	sync.OverwriteSyncedState(true)
	log.Infof("starting node in bootstrapping mode")
}

func run(_ *node.Plugin) {

	messageSpammer := spammer.New(issuer.IssuePayload)
	issuancePeriodSec := config.Node().GetInt(CfgBootstrapInitialIssuanceTimePeriodSec)
	timeUnit := config.Node().GetInt(CfgBootstrapTimeUnit)
	if timeUnit <= 0 {
		log.Panicf("Invalid Bootsrap time unit: %d seconds", timeUnit)
	}
	issuancePeriod := time.Duration(issuancePeriodSec) * time.Second

	// issue messages on top of the genesis
	if err := daemon.BackgroundWorker("Bootstrapping-Issuer", func(shutdownSignal <-chan struct{}) {
		messageSpammer.Start(initialIssuanceRate, time.Duration(timeUnit)*time.Second)
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
	}, shutdown.PriorityBootstrap); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
