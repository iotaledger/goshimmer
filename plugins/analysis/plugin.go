package analysis

import (
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/analysis/client"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface"
)

var PLUGIN = node.NewPlugin("Analysis", node.Enabled, configure, run)

func configure(plugin *node.Plugin) {
	if *server.SERVER_PORT.Value != 0 {
		webinterface.Configure(plugin)
		server.Configure(plugin)

		daemon.Events.Shutdown.Attach(events.NewClosure(func() {
			server.Shutdown(plugin)
		}))
	}
}

func run(plugin *node.Plugin) {
	if *server.SERVER_PORT.Value != 0 {
		webinterface.Run(plugin)
		server.Run(plugin)
	} else {
		plugin.Node.LogSuccess("Node", "Starting Plugin: Analysis ... server is disabled (server-port is 0)")
	}

	if *client.SERVER_ADDRESS.Value != "" {
		client.Run(plugin)
	} else {
		plugin.Node.LogSuccess("Node", "Starting Plugin: Analysis ... client is disabled (server-address is empty)")
	}
}
