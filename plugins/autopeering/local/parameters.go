package local

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_ADDRESS = "autopeering.address"
	CFG_PORT    = "autopeering.port"
	CFG_SEED    = "autopeering.seed"
)

func init() {
	flag.String(CFG_ADDRESS, "0.0.0.0", "address to bind for incoming peering requests")
	flag.Int(CFG_PORT, 14626, "udp port for incoming peering requests")
	flag.BytesBase64(CFG_SEED, nil, "private key seed used to derive the node identity; optional Base64 encoded 256-bit string")
}
