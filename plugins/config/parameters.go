package config

import (
	flag "github.com/spf13/pflag"
)

const (
	DisablePlugins = "node.disablePlugins"
	EnablePlugins  = "node.enablePlugins"
)

func init() {
	flag.StringSlice(DisablePlugins, nil, "a list of plugins that shall be disabled")
	flag.StringSlice(EnablePlugins, nil, "a list of plugins that shall be enabled")
}
