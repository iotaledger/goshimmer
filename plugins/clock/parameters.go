package clock

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the clock plugin.
type ParametersDefinition struct {
	// NTPPools defines the config flag of the NTP pools.
	NTPPools []string `default:"0.pool.ntp.org,1.pool.ntp.org,2.pool.ntp.org" usage:"list of NTP pools to synchronize time from"`
}

// Parameters contains the configuration parameters of the clock plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "clock")
}
