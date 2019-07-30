package parameters

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
	ADDRESS         = parameter.AddString("AUTOPEERING/ADDRESS", "0.0.0.0", "address to bind for incoming peering requests")
	ENTRY_NODES     = parameter.AddString("AUTOPEERING/ENTRY_NODES", "7f7a876a4236091257e650da8dcf195fbe3cb625@159.69.158.51:14626", "list of trusted entry nodes for auto peering")
	PORT            = parameter.AddInt("AUTOPEERING/PORT", 14626, "tcp port for incoming peering requests")
	ACCEPT_REQUESTS = parameter.AddBool("AUTOPEERING/ACCEPT_REQUESTS", true, "accept incoming autopeering requests")
	SEND_REQUESTS   = parameter.AddBool("AUTOPEERING/SEND_REQUESTS", true, "send autopeering requests")
)
