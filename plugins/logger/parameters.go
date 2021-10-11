package logger

import "github.com/iotaledger/hive.go/configuration"

// ParametersDefinition contains the definition of configuration parameters used by the logger plugin.
type ParametersDefinition struct {
	// Level defines the logger's level.
	Level string `default:"info" usage:"log level"`

	// DisableCaller defines whether to disable caller info.
	DisableCaller bool `default:"false" usage:"disable caller info in log"`

	// DisableStacktrace defines whether to disable stack trace info.
	DisableStacktrace bool `default:"false" usage:"disable stack trace in log"`

	// Encoding defines the logger's encoding.
	Encoding string `default:"console" usage:"log encoding"`

	// OutputPaths defines the logger's output paths.
	OutputPaths []string `default:"stdout,goshimmer.log" usage:"log output paths"`

	// DisableEvents defines whether to disable logger events.
	DisableEvents bool `default:"true" usage:"disable logger events"`
}

// Parameters contains the configuration parameters of the logger plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "logger")
}
