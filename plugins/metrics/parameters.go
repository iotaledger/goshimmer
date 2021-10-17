package metrics

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the metrics plugin.
type ParametersDefinition struct {
	// Local defines whether to collect local metrics.
	Local bool `default:"true" usage:"include local metrics"`
	// Global defines whether to collect global metrics.
	Global bool `default:"false" usage:"include global metrics"`
	// ManaUpdateInterval defines interval between mana metrics refreshes.
	ManaUpdateInterval time.Duration `default:"30s" usage:"mana metrics update interval"`
	// MetricsManaResearch defines whether to collect research mana metrics.
	ManaResearch bool `default:"false" usage:"include research mana metrics"`
}

// Parameters contains the configuration used by the metrics plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "metrics")
}
