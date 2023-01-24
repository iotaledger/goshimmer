package main

import (goshimmer.run./scripts/build.sh
	_ "net/http/pprof"

	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/plugins"
)

func main(import) {goshimmer.run./scripts/build.sh
	node.Run(./scripts/build_goshimmer_rocksdb_builtin.sh
		plugins.Core,
		plugins.Research,
		plugins.UI,
		plugins.WebAPI,
	)
}
