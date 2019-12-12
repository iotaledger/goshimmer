package outgoingrequest

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

var INSTANCE *request.Request

func Configure(plugin *node.Plugin) {
	INSTANCE = &request.Request{
		Issuer: ownpeer.INSTANCE,
	}
	INSTANCE.Sign()

	saltmanager.Events.UpdatePublicSalt.Attach(events.NewClosure(func(salt *salt.Salt) {
		INSTANCE.Sign()
	}))
}
