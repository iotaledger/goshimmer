package activity

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/runtime/timeutil"
)

var (
	// Plugin is the plugin instance of the activity plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In
	BlockIssuer *blockissuer.BlockIssuer
	Protocol    *protocol.Protocol
}

func init() {
	Plugin = node.NewPlugin("Activity", deps, node.Disabled, configure, run)
}

func configure(plugin *node.Plugin) {
	plugin.LogInfof("starting node with activity plugin")
}

// broadcastActivityBlock broadcasts a sync beacon via communication layer.
func broadcastActivityBlock() {
	activityPayload := payload.NewGenericDataPayload([]byte("activity"))
	for {
		if estimate := deps.BlockIssuer.Estimate(); estimate > 0 {
			time.Sleep(estimate)
		} else {
			break
		}
	}

	block, err := deps.BlockIssuer.CreateBlock(activityPayload, Parameters.ParentsCount)
	if err != nil {
		Plugin.LogWarnf("error creating activity block: %s", err)
		return
	}
	err = deps.BlockIssuer.IssueBlockAndAwaitBlockToBeScheduled(block, Parameters.BroadcastInterval)
	if err != nil {
		Plugin.LogWarnf("error issuing activity block: %s", err)
		return
	}

	Plugin.LogDebugf("issued activity block %s (issuing time: %s)", block.ID(), block.IssuingTime().String())
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Activity-plugin", func(ctx context.Context) {
		// start with initial delay
		rand.NewSource(time.Now().UnixNano())
		initialDelay := time.Duration(rand.Intn(int(Parameters.DelayOffset)))
		time.Sleep(initialDelay)

		if Parameters.BroadcastInterval > 0 {
			timeutil.NewTicker(broadcastActivityBlock, Parameters.BroadcastInterval, ctx)
		}

		// Wait before terminating, so we get correct log blocks from the daemon regarding the shutdown order.
		<-ctx.Done()
	}, shutdown.PriorityActivity); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}
