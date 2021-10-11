package config

import (
	"github.com/iotaledger/hive.go/configuration"
)

// The following flags are only for text support.
const (
	CfgDisablePlugins = "node.disablePlugins"
	CfgEnablePlugins  = "node.enablePlugins"
)

// ParametersDefinition contains the definition of configuration parameters used by the config plugin.
type ParametersDefinition struct {
	// EnablePlugins is the flag to manually enable node plugins.
	EnablePlugins []string `usage:"a list of plugins that shall be enabled"`

	// DisablePlugins is the flag to manually disable node plugins.
	DisablePlugins []string `usage:"a list of plugins that shall be disabled"`
}

// Parameters contains the configuration parameters of the config plugin.
var Parameters = &ParametersDefinition{}

// do not change "node" to "config" since this name is used already.
func init() {
	configuration.BindParameters(Parameters, "node")
}
