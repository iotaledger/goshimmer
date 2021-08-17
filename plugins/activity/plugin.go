package activity

import (
	"math/rand"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

var (
	// Plugin is the plugin instance of the activity plugin.
	Plugin *node.Plugin

	deps dependencies
)

type dependencies struct {
	dig.In
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin("Activity", node.Disabled, configure, run)
}

// configure events
func configure(_ *node.Plugin) {
	dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	})

	Plugin.LogInfof("starting node with activity plugin")
}

// broadcastActivityMessage broadcasts a sync beacon via communication layer.
func broadcastActivityMessage() {
	activityPayload := payload.NewGenericDataPayload([]byte("activity"))
	msg, err := deps.Tangle.IssuePayload(activityPayload, Parameters.ParentsCount)
	if err != nil {
		Plugin.LogWarnf("error issuing activity message: %s", err)
		return
	}

	Plugin.LogDebugf("issued activity message %s", msg.ID())
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Activity-plugin", func(shutdownSignal <-chan struct{}) {
		// start with initial delay
		rand.NewSource(time.Now().UnixNano())
		initialDelay := rand.Intn(Parameters.DelayOffset)
		time.Sleep(time.Duration(initialDelay) * time.Second)

		if Parameters.BroadcastIntervalSec > 0 {
			timeutil.NewTicker(broadcastActivityMessage, time.Duration(Parameters.BroadcastIntervalSec)*time.Second, shutdownSignal)
		}

		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
		<-shutdownSignal
	}, shutdown.PriorityActivity); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}
