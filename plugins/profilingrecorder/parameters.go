package profilingrecorder

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of the parameters used by the profiling plugin.
type ParametersDefinition struct {
	// OutputPath is the path where the profiling data is stored.
	OutputPath string `default:"./profiles" usage:"Output path for profiling records."`
	// Capacity defines the capacity of the profiling plugin.
	Capacity int `default:"120" usage:"Capacity of the profiling records cache."`
	// Interval defines the interval in which the profiling data is recorded.
	Interval time.Duration `default:"1m" usage:"Interval for profiling records."`
	// Duration defines the duration of the profiling data.
	Duration time.Duration `default:"30s" usage:"Duration for profiling records."`
}

// Parameters contains the configuration used by the profiling plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "profilingRecorder")
}
