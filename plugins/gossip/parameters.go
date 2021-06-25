package gossip

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// Parameters contains the configuration parameters used by the gossip plugin.
var Parameters = struct {
	// NetworkVersion defines the config flag of the network version.
	Port int `default:"14666" usage:"tcp port for gossip connection"`

	// TipsBroadcaster describes configuration parameters related to the tips broadcaster.
	TipsBroadcaster struct {
		// Enable defines whether the tips broadcaster is enabled.
		Enable bool `default:"true" usage:"enable/disable tip re-broadcasting"`

		// Interval defines the interval to broadcast tips.
		Interval time.Duration `default:"10s" usage:"the interval in which the oldest known tip is re-broadcast"`
	}
}{}

func init() {
	configuration.BindParameters(&Parameters, "gossip")
}
