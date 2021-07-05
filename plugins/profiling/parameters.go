package profiling

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the profiling plugin.
type ParametersDefinition struct {
	// BindAddress defines the bind address for the pprof server.
	BindAddress string `default:"127.0.0.1:6061" usage:"bind address for the pprof server"`
}

// Parameters contains the configuration used by the profiling plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "profiling")
}
