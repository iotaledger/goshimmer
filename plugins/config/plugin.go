package config

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/spf13/viper"

	flag "github.com/spf13/pflag"
)

var (
	// define the plugin as a placeholder, so the init methods get executed accordingly
	PLUGIN = node.NewPlugin("Config", node.Enabled)

	// flags<
	configName    = flag.StringP("config", "c", "config", "Filename of the config file without the file extension")
	configDirPath = flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")

	// viper
	Node *viper.Viper

	// logger
	defaultLoggerConfig = logger.Config{
		Level:             "info",
		DisableCaller:     false,
		DisableStacktrace: false,
		Encoding:          "console",
		OutputPaths:       []string{"goshimmer.log"},
		DisableEvents:     false,
	}
)

func Init() {
	PLUGIN.Events.Init.Trigger(PLUGIN)
}

func init() {
	// set the default logger config
	Node = viper.New()
	Node.SetDefault(logger.ViperKey, defaultLoggerConfig)

	PLUGIN.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := fetch(false); err != nil {
			panic(err)
		}
	}))
}

// fetch fetches config values from a dir defined via CLI flag --config-dir (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func fetch(printConfig bool, ignoreSettingsAtPrint ...[]string) error {
	flag.Parse()
	err := parameter.LoadConfigFile(Node, *configDirPath, *configName, true, true)
	if err != nil {
		return err
	}

	if printConfig {
		parameter.PrintConfig(Node, ignoreSettingsAtPrint...)
	}

	for _, pluginName := range Node.GetStringSlice(node.CFG_DISABLE_PLUGINS) {
		node.DisabledPlugins[node.GetPluginIdentifier(pluginName)] = true
	}
	for _, pluginName := range Node.GetStringSlice(node.CFG_ENABLE_PLUGINS) {
		node.EnabledPlugins[node.GetPluginIdentifier(pluginName)] = true
	}

	return nil
}
