package acceptedneighbors

import "github.com/iotaledger/hive.go/node"

func Configure(plugin *node.Plugin) {
	configureOwnDistance()
	configureFurthestNeighbor()
}
