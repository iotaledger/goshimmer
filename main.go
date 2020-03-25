package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/pluginmgr/core"
	"github.com/iotaledger/goshimmer/pluginmgr/research"
	"github.com/iotaledger/goshimmer/pluginmgr/ui"
	"github.com/iotaledger/goshimmer/pluginmgr/webapi"

	"github.com/iotaledger/hive.go/node"
)

func main() {
	go http.ListenAndServe("localhost:6061", nil) // pprof Server for Debbuging Mutexes

	node.Run(
		core.PLUGINS,
		research.PLUGINS,
		ui.PLUGINS,
		webapi.PLUGINS,
	)
}
