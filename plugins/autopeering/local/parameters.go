package local

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_BIND     = "network.bindAddress"
	CFG_EXTERNAL = "network.externalAddress"
	CFG_PORT     = "autopeering.port"
	CFG_SEED     = "autopeering.seed"
)

func init() {
	flag.String(CFG_BIND, "0.0.0.0", "bind address for global services such as autopeering and gossip")
	flag.String(CFG_EXTERNAL, "auto", "external IP address under which the node is reachable; or 'auto' to determine it automatically")
	flag.Int(CFG_PORT, 14626, "UDP port for incoming peering requests")
	flag.BytesBase64(CFG_SEED, nil, "private key seed used to derive the node identity; optional Base64 encoded 256-bit string")
}
