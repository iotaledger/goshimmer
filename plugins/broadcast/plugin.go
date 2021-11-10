package broadcast

import (
	"context"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/broadcast/server"
)

const (
	pluginName = "Broadcast"
)

var (
	// Plugin defines the plugin instance of the broadcast plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin(pluginName, deps, node.Disabled, run)
	configuration.BindParameters(Parameters, "Broadcast")
}

// ParametersDefinition contains the configuration parameters used by the plugin.
type ParametersDefinition struct {
	// BindAddress defines on which address the broadcast plugin should listen on.
	BindAddress string `default:"0.0.0.0:5050" usage:"the bind address for the broadcast plugin"`
}

// Parameters contains the configuration parameters of the broadcast plugin.
var Parameters = &ParametersDefinition{}

func run(_ *node.Plugin) {
	// Server to connect to.
	Plugin.LogInfof("Starting Broadcast plugin on %s", Parameters.BindAddress)
	if err := daemon.BackgroundWorker("Broadcast worker", func(ctx context.Context) {
		if err := server.Listen(Parameters.BindAddress, Plugin, ctx.Done()); err != nil {
			Plugin.LogError("Failed to start Broadcast server: %v", err)
		}
		<-ctx.Done()
	}); err != nil {
		Plugin.LogFatalf("Failed to start Broadcast daemon: %v", err)
	}

	// Get Messages from node.
	notifyNewMsg := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			go func() {
				server.Broadcast([]byte(message.String()))
			}()
		})
	})

	if err := daemon.BackgroundWorker("Broadcast[MsgUpdater]", func(ctx context.Context) {
		deps.Tangle.Storage.Events.MessageStored.Attach(notifyNewMsg)
		<-ctx.Done()
		Plugin.LogInfof("Stopping Broadcast...")
		deps.Tangle.Storage.Events.MessageStored.Detach(notifyNewMsg)
		Plugin.LogInfof("Stopping Broadcast... \tDone")
	}, shutdown.PriorityBroadcast); err != nil {
		Plugin.LogError("Failed to start as daemon: %s", err)
	}
}
