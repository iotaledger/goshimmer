package activity

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	delayOffset = 10
)

var (
	// plugin is the plugin instance of the activity plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("Activity", node.Disabled, configure, run)
	})
	return plugin
}

// configure events
func configure(_ *node.Plugin) {
	plugin.LogInfof("starting node with activity plugin")
}

// broadcastActivityMessage broadcasts a sync beacon via communication layer.
func broadcastActivityMessage() {
	activityPayload := payload.NewGenericDataPayload([]byte("activity"))
	msg, err := messagelayer.Tangle().IssuePayload(activityPayload)
	if err != nil {
		plugin.LogWarnf("error issuing activity message: %s", err)
		return
	}

	plugin.LogDebugf("issued activity message %s", msg.ID())
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Activity-plugin", func(shutdownSignal <-chan struct{}) {
		// start with initial delay
		rand.NewSource(time.Now().UnixNano())
		initialDelay := rand.Intn(delayOffset)
		time.Sleep(time.Duration(initialDelay) * time.Second)

		timeutil.NewTicker(broadcastActivityMessage, time.Duration(Parameters.BroadcastIntervalSec)*time.Second, shutdownSignal)

		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
		<-shutdownSignal
	}, shutdown.PriorityActivity); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}
