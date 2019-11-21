package parameters

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
	ADDRESS         = parameter.AddString("AUTOPEERING/ADDRESS", "0.0.0.0", "address to bind for incoming peering requests")
	ENTRY_NODES     = parameter.AddString("AUTOPEERING/ENTRY_NODES", "qLsSGVTFm3WLJmFgTntHJM/NoFcvN6LpZl2/bFMv2To=@116.202.49.178:14626", "list of trusted entry nodes for auto peering")
	PORT            = parameter.AddInt("AUTOPEERING/PORT", 14626, "tcp port for incoming peering requests")
	ACCEPT_REQUESTS = parameter.AddBool("AUTOPEERING/ACCEPT_REQUESTS", true, "accept incoming autopeering requests")
	SEND_REQUESTS   = parameter.AddBool("AUTOPEERING/SEND_REQUESTS", true, "send autopeering requests")
)
