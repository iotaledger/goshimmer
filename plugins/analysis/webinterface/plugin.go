package webinterface

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/httpserver"
	"github.com/iotaledger/goshimmer/plugins/analysis/webinterface/recordedevents"
)

func Configure(plugin *node.Plugin) {
	httpserver.Configure(plugin)
	recordedevents.Configure(plugin)
}

func Run(plugin *node.Plugin) {
	httpserver.Run(plugin)
}
