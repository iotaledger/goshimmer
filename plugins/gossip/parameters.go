package gossip

import (
	"time"

	flag "github.com/spf13/pflag"
)

const (
	// CfgGossipPort defines the config flag of the gossip port.
	CfgGossipPort = "gossip.port"
	// CfgGossipAgeThreshold defines the maximum age (time since reception) of a message to be gossiped.
	CfgGossipAgeThreshold = "gossip.ageThreshold"
	// CfgGossipTipsBroadcastInterval the interval in which the oldest known tip is re-broadcast.
	CfgGossipTipsBroadcastInterval = "gossip.tipsBroadcaster.interval"
	// CfgGossipDisableAutopeering if set to true the autopeering layer won't manage neighbors in the gossip layer.
	CfgGossipDisableAutopeering = "gossip.disableAutopeering"
	// CfgGossipManualNeighbors list of peers that will be used as neighbors in the gossip layer.
	CfgGossipManualNeighbors = "gossip.manualNeighbors"
)

func init() {
	flag.Int(CfgGossipPort, 14666, "tcp port for gossip connection")
	flag.Duration(CfgGossipAgeThreshold, 5*time.Second, "message age threshold for gossip")
	flag.Duration(CfgGossipTipsBroadcastInterval, 10*time.Second, "the interval in which the oldest known tip is re-broadcast")
	flag.Bool(CfgGossipDisableAutopeering, false, "if set to true the autopeering layer won't manage neighbors in the gossip layer")
}
