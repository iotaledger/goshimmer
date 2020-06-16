package autopeering

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgEntryNodes defines the config flag of the entry nodes.
	CfgEntryNodes = "autopeering.entryNodes"

	// CfgOutboundUpdateIntervalMs time after which out neighbors are updated.
	CfgOutboundUpdateIntervalMs = "autopeering.outboundUpdateIntervalMs"
)

func init() {
	flag.StringSlice(CfgEntryNodes, []string{"V8LYtWWcPYYDTTXLeIEFjJEuWlsjDiI0+Pq/Cx9ai6g=@116.202.49.178:14626"}, "list of trusted entry nodes for auto peering")
	flag.Int(CfgOutboundUpdateIntervalMs, 10, "time after which out neighbors are updated")
}
