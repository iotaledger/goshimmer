package config

import (
	flag "github.com/spf13/pflag"
)

const (
	DISABLE_PLUGINS = "node.disablePlugins"
	ENABLE_PLUGINS  = "node.enablePlugins"
)

func init() {
	flag.StringSlice(DISABLE_PLUGINS, nil, "a list of plugins that shall be disabled")
	flag.StringSlice(ENABLE_PLUGINS, nil, "a list of plugins that shall be enabled")
}
