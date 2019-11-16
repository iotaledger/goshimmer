package protocol

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/drop"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

func createIncomingDropProcessor(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(drop *drop.Drop) {
		log.Debugf("received drop message from %s", drop.Issuer.String())

		chosenneighbors.INSTANCE.Remove(drop.Issuer.GetIdentity().StringIdentifier)
		acceptedneighbors.INSTANCE.Remove(drop.Issuer.GetIdentity().StringIdentifier)
	})
}
