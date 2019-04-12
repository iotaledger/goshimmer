package outgoingrequest

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
)

var INSTANCE *request.Request

func init() {
    INSTANCE = &request.Request{
        Issuer: ownpeer.INSTANCE,
    }
    INSTANCE.Sign()

    saltmanager.Events.UpdatePublicSalt.Attach(func(salt *salt.Salt) {
        INSTANCE.Sign()
    })
}
