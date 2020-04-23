package webinterface

import (
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/httpserver"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/recordedevents"
	"github.com/iotaledger/hive.go/node"
)

// Configure configures the plugin.
func Configure(plugin *node.Plugin) {
	httpserver.Configure()
	recordedevents.Configure(plugin)
}

// Run runs the plugin.
func Run(plugin *node.Plugin) {
	httpserver.Run()
	recordedevents.Run()
}
