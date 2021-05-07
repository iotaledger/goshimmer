package activity

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"

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
func broadcastActivityMessage() (doneSignal chan struct{}) {
	doneSignal = make(chan struct{}, 1)
	go func() {
		defer close(doneSignal)

		activityPayload := payload.NewGenericDataPayload([]byte("activity"))
		msg, err := messagelayer.Tangle().IssuePayload(activityPayload)
		if err != nil {
			plugin.LogWarnf("error issuing activity message: %s", err)
			return
		}

		plugin.LogDebugf("issued activity message %s", msg.ID())
	}()

	return
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Activity-plugin", func(shutdownSignal <-chan struct{}) {
		// start with initial delay
		rand.NewSource(time.Now().UnixNano())
		initialDelay := rand.Intn(delayOffset)
		time.Sleep(time.Duration(initialDelay) * time.Second)

		ticker := time.NewTicker(time.Duration(Parameters.BroadcastIntervalSec) * time.Second)
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
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}
