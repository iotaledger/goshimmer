package saltmanager

import "github.com/iotaledger/goshimmer/plugins/autopeering/salt"

type SaltConsumer = func(salt *salt.Salt)
