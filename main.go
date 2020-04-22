package main

import (
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/pluginmgr/core"
	"github.com/iotaledger/goshimmer/pluginmgr/research"
	"github.com/iotaledger/goshimmer/pluginmgr/ui"
	"github.com/iotaledger/goshimmer/pluginmgr/webapi"

	"github.com/iotaledger/hive.go/node"
)

func main() {
	node.Run(
		core.PLUGINS,
		research.PLUGINS,
		ui.PLUGINS,
		webapi.PLUGINS,
	)
}
