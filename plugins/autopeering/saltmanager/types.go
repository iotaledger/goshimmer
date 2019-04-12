package saltmanager

import "github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"

type SaltConsumer = func(salt *salt.Salt)
