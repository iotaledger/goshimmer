package analysis

import (
	"github.com/iotaledger/goshimmer/plugins/analysis/client"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the analysis plugin.
const PluginName = "Analysis"

var (
	// Plugin is the plugin instance of the analysis plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
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
