package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// DisablePlugins contains the name of the parameter that allows to manually disable node plugins.
	DisablePlugins = "node.disablePlugins"

	// EnablePlugins contains the name of the parameter that allows to manually enable node plugins.
	EnablePlugins = "node.enablePlugins"
)

func init() {
	flag.StringSlice(DisablePlugins, nil, "a list of plugins that shall be disabled")
	flag.StringSlice(EnablePlugins, nil, "a list of plugins that shall be enabled")
}
