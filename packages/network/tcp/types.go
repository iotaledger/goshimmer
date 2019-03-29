package tcp

import "github.com/iotaledger/goshimmer/packages/network"

type Callback = func()

type ErrorConsumer = func(e error)

type PeerConsumer = func(peer network.Connection)
