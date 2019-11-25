package node

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_LOG_LEVEL       = "node.logLevel"
	CFG_DISABLE_PLUGINS = "node.disablePlugins"
	CFG_ENABLE_PLGUINS  = "node.enablePlugins"
)

func init() {
	flag.Int(CFG_LOG_LEVEL, LOG_LEVEL_INFO, "controls the log types that are shown")
	flag.String(CFG_DISABLE_PLUGINS, "", "a list of plugins that shall be disabled")
	flag.String(CFG_ENABLE_PLGUINS, "", "a list of plugins that shall be enabled")
}
