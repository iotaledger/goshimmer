package analysis

import (
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/plugins/analysis/client"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Analysis", node.Enabled, configure, run)
var log = logger.NewLogger("Analysis")

func configure(plugin *node.Plugin) {
	if parameter.NodeConfig.GetInt(server.CFG_SERVER_PORT) != 0 {
		webinterface.Configure(plugin)
		server.Configure(plugin)
	}
}

func run(plugin *node.Plugin) {
	if parameter.NodeConfig.GetInt(server.CFG_SERVER_PORT) != 0 {
		webinterface.Run(plugin)
		server.Run(plugin)
	} else {
		log.Info("Starting Plugin: Analysis ... server is disabled (server-port is 0)")
	}

	if parameter.NodeConfig.GetString(client.CFG_SERVER_ADDRESS) != "" {
		client.Run(plugin)
	} else {
		log.Info("Starting Plugin: Analysis ... client is disabled (server-address is empty)")
	}
}
