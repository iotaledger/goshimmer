package parameters

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_ADDRESS         = "autopeering.address"
	CFG_ENTRY_NODES     = "autopeering.entryNodes"
	CFG_PORT            = "autopeering.port"
	CFG_ACCEPT_REQUESTS = "autopeering.acceptRequests"
	CFG_SEND_REQUESTS   = "autopeering.sendRequests"
)

func init() {
	flag.String(CFG_ADDRESS, "0.0.0.0", "address to bind for incoming peering requests")
	flag.String(CFG_ENTRY_NODES, "7f7a876a4236091257e650da8dcf195fbe3cb625@159.69.158.51:14626", "list of trusted entry nodes for auto peering")
	flag.Int(CFG_PORT, 14626, "tcp port for incoming peering requests")
	flag.Bool(CFG_ACCEPT_REQUESTS, true, "accept incoming autopeering requests")
	flag.Bool(CFG_SEND_REQUESTS, true, "send autopeering requests")
}
