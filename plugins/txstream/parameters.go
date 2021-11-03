package txstream

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the txStream plugin.
type ParametersDefinition struct {
	// BindAddress defines the bind address for the txStream server.
	BindAddress string `default:"0.0.0.0:5000" usage:"the bind address for the txStream plugin"`
}

// Parameters contains the configuration used by the txStream plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "txStream")
}
