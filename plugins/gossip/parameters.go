package gossip

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgGossipPort defines the config flag of the gossip port.
	CfgGossipPort = "gossip.port"
)

func init() {
	flag.Int(CfgGossipPort, 14666, "tcp port for gossip connection")
}
