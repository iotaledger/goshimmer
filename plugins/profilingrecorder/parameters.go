package profilingrecorder

import "github.com/iotaledger/goshimmer/plugins/config"

// ParametersDefinition contains the definition of the parameters used by the profiling plugin.
type ParametersDefinition struct {
	// BindAddress defines the bind address for the pprof server.
	OutputPath string `default:"./profiles" usage:"Output path for profiling records."`
}

// Parameters contains the configuration used by the profiling plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "profilingRecorder")
}
