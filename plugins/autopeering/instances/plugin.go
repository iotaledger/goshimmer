package instances

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighborcandidates"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/entrynodes"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
)

func Configure(plugin *node.Plugin) {
    ownpeer.Configure(plugin)
    entrynodes.Configure(plugin)
    knownpeers.Configure(plugin)
    neighborhood.Configure(plugin)
    outgoingrequest.Configure(plugin)
    chosenneighborcandidates.Configure(plugin)
}
