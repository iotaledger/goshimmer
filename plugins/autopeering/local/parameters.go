package local

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgBind defines the config flag of the network bind address.
	CfgBind = "network.bindAddress"
	// CfgExternal defines the config flag of the network external address.
	CfgExternal = "network.externalAddress"
	// CfgPort defines the config flag of the autopeering port.
	CfgPort = "autopeering.port"
	// CfgSeed defines the config flag of the autopeering private key seed.
	CfgSeed = "autopeering.seed"
)

func init() {
	flag.String(CfgBind, "0.0.0.0", "bind address for global services such as autopeering and gossip")
	flag.String(CfgExternal, "auto", "external IP address under which the node is reachable; or 'auto' to determine it automatically")
	flag.Int(CfgPort, 14626, "UDP port for incoming peering requests")
	flag.BytesBase64(CfgSeed, nil, "private key seed used to derive the node identity; optional Base64 encoded 256-bit string")
}
