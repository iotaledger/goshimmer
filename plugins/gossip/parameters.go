package gossip

import (
	"time"

	flag "github.com/spf13/pflag"
)

const (
	// CfgGossipPort defines the config flag of the gossip port.
	CfgGossipPort = "gossip.port"

	// CfgGossipTipsBroadcastInterval the interval in which the oldest known tip is re-broadcasted.
	CfgGossipTipsBroadcastInterval = "gossip.tipsBroadcaster.interval"
)

func init() {
	flag.Int(CfgGossipPort, 14666, "tcp port for gossip connection")
	flag.Duration(CfgGossipTipsBroadcastInterval, 10*time.Second, "the interval in which the oldest known tip is re-broadcasted")
}
