package syncbeacon

import (
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
	"sync"
	"time"
)

const (
	// PluginName is the plugin name of the sync beacon plugin.
	PluginName = "Sync Beacon"

	// CfgSyncBeaconBroadcastIntervalSec is the interval in seconds at which the node broadcasts its sync status.
	CfgSyncBeaconBroadcastIntervalSec = "syncbeacon.broadcastInterval"
)

func init() {
	flag.Int(CfgSyncBeaconBroadcastIntervalSec, 30, "the interval at which the node will broadcast ist sync status")
}

var (
	// plugin is the plugin instance of the sync beacon plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

// configure events
func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

// broadcastSyncBeaconPayload broadcasts a sync beacon via communication layer.
func broadcastSyncBeaconPayload() {
	syncBeaconPayload := NewSyncBeaconPayload(time.Now().UnixNano())
	msg, err := messagelayer.MessageFactory().IssuePayload(syncBeaconPayload)
	if err != nil {
		log.Infof("error issuing sync beacon. %w", err)
	} else {
		log.Infof("issued sync beacon %s", msg.Id())
	}
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Sync-Beacon", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(config.Node().GetDuration(CfgSyncBeaconBroadcastIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				broadcastSyncBeaconPayload()
			case <-shutdownSignal:
				return
			}
		}
	}, shutdown.PrioritySynchronization); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
