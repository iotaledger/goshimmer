package outgoingrequest

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
)

var INSTANCE *request.Request

func Configure(plugin *node.Plugin) {
	INSTANCE = &request.Request{
		Issuer: ownpeer.INSTANCE,
	}
}
