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
	flag.StringSlice(CfgEntryNodes, []string{"2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@ressims.iota.cafe:15626", "5EDH4uY78EA6wrBkHHAVBWBMDt7EcksRq6pjzipoW15B@entrynode.alphanet.tanglebay.org:14656"}, "list of trusted entry nodes for auto peering")
	flag.Uint32(CfgNetworkVersion, 8, "autopeering network version")
}
