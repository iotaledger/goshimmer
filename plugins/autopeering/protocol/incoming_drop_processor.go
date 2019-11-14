package protocol

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/drop"
	"github.com/iotaledger/hive.go/events"
)

func createIncomingDropProcessor(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(drop *drop.Drop) {
		plugin.LogDebug("received drop message from " + drop.Issuer.String())

		chosenneighbors.INSTANCE.Remove(drop.Issuer.GetIdentity().StringIdentifier)
		acceptedneighbors.INSTANCE.Remove(drop.Issuer.GetIdentity().StringIdentifier)
	})
}
