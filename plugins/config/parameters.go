package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgDisablePlugins contains the name of the parameter that allows to manually disable node plugins.
	CfgDisablePlugins = "node.disablePlugins"

	// CfgEnablePlugins contains the name of the parameter that allows to manually enable node plugins.
	CfgEnablePlugins = "node.enablePlugins"
)

func init() {
	flag.StringSlice(CfgDisablePlugins, nil, "a list of plugins that shall be disabled")
	flag.StringSlice(CfgEnablePlugins, nil, "a list of plugins that shall be enabled")
}
