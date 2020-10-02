package main

import (
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/plugins"
	"github.com/iotaledger/hive.go/node"
)

func main() {
	node.Run(
		plugins.Core,
		plugins.Research,
		plugins.Ui,
		plugins.WebApi,
	)
}
