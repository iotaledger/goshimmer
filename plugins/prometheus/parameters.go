package prometheus

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the prometheus plugin.
type ParametersDefinition struct {
	// BindAddress defines the bind address for the Prometheus exporter server.
	BindAddress string `default:"0.0.0.0:9311" usage:"bind address on which the Prometheus exporter server"`
	// GoMetrics defines whether to include Go metrics.
	GoMetrics bool `default:"false" usage:"include go metrics"`
	// ProcessMetrics defines whether to include process metrics.
	ProcessMetrics bool `default:"true" usage:"include process metrics"`
	// PromhttpMetrics defines whether to include promhttp metrics.
	PromhttpMetrics bool `default:"false" usage:"include promhttp metrics"`
	// WorkerpoolMetrics defines whether to include workerpool metrics.
	WorkerpoolMetrics bool `default:"false" usage:"include workerpool metrics"`
}

// Parameters contains the configuration used by the prometheus plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "prometheus")
}
