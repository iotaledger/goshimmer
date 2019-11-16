package chosenneighbors

import (
	"github.com/iotaledger/hive.go/node"
)

func Configure(plugin *node.Plugin) {
	configureCandidates()
	configureOwnDistance()
	configureFurthestNeighbor()
}
