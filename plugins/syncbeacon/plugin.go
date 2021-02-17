package syncbeacon

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/syncbeacon/payload"
	"github.com/iotaledger/goshimmer/plugins/syncbeaconfollower"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName is the plugin name of the sync beacon plugin.
	PluginName = "SyncBeacon"

	// CfgSyncBeaconBroadcastIntervalSec is the interval in seconds at which the node broadcasts its sync status.
	CfgSyncBeaconBroadcastIntervalSec = "syncbeacon.broadcastInterval"

	// CfgSyncBeaconStartSynced defines whether to start the sync beacon in synced mode so it can issue an initial sync beacon message.
	CfgSyncBeaconStartSynced = "syncbeacon.startSynced"
)

func init() {
	flag.Int(CfgSyncBeaconBroadcastIntervalSec, 30, "the interval at which the node will broadcast its sync status")
	flag.Bool(CfgSyncBeaconStartSynced, false, "set node to start as synced so it can issue an initial sync beacon message")
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

	log.Infof("starting node as sync beacon")

	if config.Node().Bool(CfgSyncBeaconStartSynced) {
		log.Infof("Retrieving all the tips")
		messagelayer.Tangle().TipManager.Set(messagelayer.Tangle().Storage.RetrieveAllTips()...)

		syncbeaconfollower.OverwriteSyncedState(true)
		log.Infof("overwriting synced state to 'true'")
	}
}

// broadcastSyncBeaconPayload broadcasts a sync beacon via communication layer.
func broadcastSyncBeaconPayload() (doneSignal chan struct{}) {
	doneSignal = make(chan struct{}, 1)
	go func() {
		defer close(doneSignal)

		syncBeaconPayload := payload.NewSyncBeaconPayload(clock.SyncedTime().UnixNano())
		msg, err := issuer.IssuePayload(syncBeaconPayload, messagelayer.Tangle())

		if err != nil {
			log.Warnf("error issuing sync beacon: %w", err)
			return
		}

		log.Debugf("issued sync beacon %s", msg.ID())
	}()

	return
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Sync-Beacon", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(config.Node().Duration(CfgSyncBeaconBroadcastIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
				return
			case <-ticker.C:
				doneSignal := broadcastSyncBeaconPayload()

				select {
				case <-shutdownSignal:
					return
				case <-doneSignal:
					// continue with the next beacon
				}
			}
		}
	}, shutdown.PrioritySynchronization); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
