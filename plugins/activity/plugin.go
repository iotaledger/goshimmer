package syncbeacon

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	// PluginName is the plugin name of the activity plugin.
	PluginName = "Activity"

	// CfgActivityBroadcastIntervalSec is the interval in seconds at which the node broadcasts its activity message.
	CfgActivityBroadcastIntervalSec = "activity.broadcastInterval"
)

func init() {
	flag.Int(CfgActivityBroadcastIntervalSec, 3, "the interval at which the node will broadcast its activity message")
}

var (
	// plugin is the plugin instance of the activity plugin.
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

	log.Infof("starting node with activity plugin")
}

// broadcastActivityMessage broadcasts a sync beacon via communication layer.
func broadcastActivityMessage() (doneSignal chan struct{}) {
	doneSignal = make(chan struct{}, 1)
	go func() {
		defer close(doneSignal)

		activityPayload := payload.NewGenericDataPayload([]byte("activity"))
		msg, err := messagelayer.Tangle().IssuePayload(activityPayload)
		if err != nil {
			log.Warnf("error issuing activity message: %w", err)
			return
		}

		log.Debugf("issued activity message %s", msg.ID())
	}()

	return
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Activity-plugin", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(config.Node().Duration(CfgActivityBroadcastIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
				return
			case <-ticker.C:
				doneSignal := broadcastActivityMessage()

				select {
				case <-shutdownSignal:
					return
				case <-doneSignal:
					// continue with the next message
				}
			}
		}
	}, shutdown.PriorityActivity); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
