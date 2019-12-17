package autopeering

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_ADDRESS     = "autopeering.address"
	CFG_ENTRY_NODES = "autopeering.entryNodes"
	CFG_PORT        = "autopeering.port"
	CFG_SELECTION   = "autopeering.selection"
)

func init() {
	flag.String(CFG_ADDRESS, "0.0.0.0", "address to bind for incoming peering requests")
	flag.String(CFG_ENTRY_NODES, "V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq/Cx9ai6g=@116.202.49.178:14626", "list of trusted entry nodes for auto peering")
	flag.Int(CFG_PORT, 14626, "udp port for incoming peering requests")
	flag.Bool(CFG_SELECTION, true, "enable peer selection")
}
