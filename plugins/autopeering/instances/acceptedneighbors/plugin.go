package acceptedneighbors

import "github.com/iotaledger/goshimmer/packages/node"

func Configure(plugin *node.Plugin) {
	configureOwnDistance()
	configureFurthestNeighbor()
}
