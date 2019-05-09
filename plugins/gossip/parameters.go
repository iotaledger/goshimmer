package gossip

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
    PORT = parameter.AddInt("GOSSIP/PORT", 14666, "tcp port for gossip connection")
)
