package dashboardmetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of the parameters used by the metrics plugin.
type ParametersDefinition struct {
	// ManaUpdateInterval defines interval between mana metrics refreshes.
	ManaUpdateInterval time.Duration `default:"30s" usage:"mana metrics update interval"`
}

// Parameters contains the configuration used by the metrics plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "dashboardmetrics")
}
