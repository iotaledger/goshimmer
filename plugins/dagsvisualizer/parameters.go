package dagsvisualizer

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the dags visualizer plugin.
type ParametersDefinition struct {
	// BindAddress defines the config flag of the dags visualizer binding address.
	BindAddress string `default:"0.0.0.0:8061" usage:"the bind address of the dags visualizer"`

	// Dev defines the config flag of the  dags visualizer dev mode.
	Dev bool `default:"false" usage:"whether the dags visualizer runs in dev mode"`

	// DevBindAddress defines the config flag of the dags visualizer binding address in development mode.
	DevBindAddress string `default:"0.0.0.0:3000" usage:"the bind address of the dags visualizer in develop mode"`
}

// Parameters contains the configuration parameters of the dags visualizer plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "dagsvisualizer")
}
