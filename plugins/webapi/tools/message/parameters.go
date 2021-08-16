package message

import "github.com/iotaledger/hive.go/configuration"

const (
	// CfgExportPath the directory where exported files sit.
	CfgExportPath = "webapi.exportPath"
)

// ParametersDefinition contains the definition of configuration parameters.
type ParametersDefinition struct {
	// Export path
	CfgExportPath string `default:"." usage:"default export path"`
}

// Parameters contains the configuration parameters of the clock plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "webapi.tools.message")
}
