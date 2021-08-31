package message

import "github.com/iotaledger/hive.go/configuration"

// ParametersDefinition contains the definition of configuration parameters.
type ParametersDefinition struct {
	// Export path
	ExportPath string `default:"." usage:"default export path"`
}

// Parameters contains the configuration parameters of the web API tools endpoint plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "webAPI")
}
