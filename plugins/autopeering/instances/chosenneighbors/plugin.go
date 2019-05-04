package chosenneighbors

import (
    "github.com/iotaledger/goshimmer/packages/node"
)

func Configure(plugin *node.Plugin) {
    configureCandidates()
    configureOwnDistance()
    configureFurthestNeighbor()
}
