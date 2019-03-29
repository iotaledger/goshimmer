package parameters

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
    ADDRESS     = parameter.AddString("AUTOPEERING/ADDRESS", "0.0.0.0", "address to bind for incoming peering requests")
    UDP_PORT    = parameter.AddInt("AUTOPEERING/UDP_PORT", 14626, "udp port for incoming peering requests")
    ENTRY_NODES = parameter.AddString("AUTOPEERING/ENTRY_NODES", "tcp://82.165.29.179:14626", "list of trusted entry nodes for auto peering")
)
