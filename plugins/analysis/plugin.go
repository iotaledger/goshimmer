package analysis

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/analysis/client"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface"
	"github.com/iotaledger/goshimmer/plugins/config"
)

var PLUGIN = node.NewPlugin("Analysis", node.Enabled, configure, run)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Analysis")
	if config.Node.GetInt(server.CfgServerPort) != 0 {
		webinterface.Configure(plugin)
		server.Configure(plugin)
	}
}

func run(plugin *node.Plugin) {
	if config.Node.GetInt(server.CfgServerPort) != 0 {
		webinterface.Run(plugin)
		server.Run(plugin)
	} else {
		log.Info("Server is disabled (server-port is 0)")
	}

	if config.Node.GetString(client.CfgServerAddress) != "" {
		client.Run(plugin)
	} else {
		log.Info("Client is disabled (server-address is empty)")
	}
}
