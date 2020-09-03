package autopeering

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgEntryNodes defines the config flag of the entry nodes.
	CfgEntryNodes = "autopeering.entryNodes"
	// CfgNetworkVersion defines the config flag of the network version.
	CfgNetworkVersion = "autopeering.networkVersion"
)

func init() {
	flag.StringSlice(CfgEntryNodes, []string{"2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@ressims.iota.cafe:15626"}, "list of trusted entry nodes for auto peering")
	flag.Uint32(CfgNetworkVersion, 7, "autopeering network version")
}
