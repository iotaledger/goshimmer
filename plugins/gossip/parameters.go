package gossip

import (
	flag "github.com/spf13/pflag"
)

const (
	GOSSIP_PORT = "gossip.port"
)

func init() {
	flag.Int(GOSSIP_PORT, 14666, "tcp port for gossip connection")
}
