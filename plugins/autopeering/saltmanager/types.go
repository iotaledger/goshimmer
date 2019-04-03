package saltmanager

import "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"

type SaltConsumer = func(salt *salt.Salt)
