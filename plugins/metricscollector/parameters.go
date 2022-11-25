package metricscollector

import "time"

// ParametersDefinition contains the definition of the parameters used by the metrics plugin.
type ParametersDefinition struct {
	// BindAddress defines the bind address for the Prometheus exporter server.
	BindAddress string `default:"0.0.0.0:9311" usage:"bind address on which the Prometheus exporter server"`
	// Local defines whether to collect local metrics.
	Local bool `default:"true" usage:"include local metrics"`
	// Global defines whether to collect global metrics.
	Global bool `default:"false" usage:"include global metrics"`
	// ManaUpdateInterval defines interval between mana metrics refreshes.
	ManaUpdateInterval time.Duration `default:"30s" usage:"mana metrics update interval"`
}

// Parameters contains the configuration used by the metrics collector plugin.
var Parameters = &ParametersDefinition{}
