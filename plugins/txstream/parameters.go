package txstream

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the txstream plugin.
type ParametersDefinition struct {
	// BindAddress defines the bind address for the txstream server.
	BindAddress string `default:"0.0.0.0:5000" usage:"the bind address for the txstream plugin"`
}

// Parameters contains the configuration used by the txstream plugin
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "txstream")
}
