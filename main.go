package main

import (
	_ "net/http/pprof"

	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins"
)

// test sentence which should trigger a complaint
func main() {
	node.Run(
		plugins.Core,
		plugins.Research,
		plugins.UI,
		plugins.WebAPI,
	)
}
