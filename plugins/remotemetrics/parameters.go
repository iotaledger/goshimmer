package remotemetrics

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the remotelog plugin.
type ParametersDefinition struct {
	// MetricsLevelMetricsLevel used limit the amount of metrics sent to metrics collection service. The higher the value, the less logs is sent
	MetricsLevel uint8 `default:"1" usage:"Numeric value to limit the amount of metrics sent to metrics collection service. The higher the value, the less logs is sent"`
}

// Parameters contains the configuration used by the remotelog plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "remotemetrics")
}
