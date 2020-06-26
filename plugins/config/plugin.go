package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/parameter"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// PluginName is the name of the config plugin.
const PluginName = "Config"

var (
	// plugin is the plugin instance of the config plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once

	// flags
	configName          = flag.StringP("config", "c", "config", "Filename of the config file without the file extension")
	configDirPath       = flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")
	skipConfigAvailable = flag.Bool("skip-config", false, "Skip config file availability check")

	// Node is viper
	_node    *viper.Viper
	nodeOnce sync.Once
)

// Init triggers the Init event.
func Init() {
	plugin.Events.Init.Trigger(plugin)
}

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled)
	})
	return plugin
}

// Node gets the node.
func Node() *viper.Viper {
	nodeOnce.Do(func() {
		_node = viper.New()
	})
	return _node
}

func init() {
	// set the default logger config
	_node = Node()
	plugin = Plugin()

	plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := fetch(false); err != nil {
			if !*skipConfigAvailable {
				// we wanted a config file but it was not present
				// global logger instance is not initialized at this stage...
				fmt.Println(err.Error())
				fmt.Println("no config file present, terminating GoShimmer. please use the provided config.default.json to create a config.json.")
				// daemon is not running yet, so we just exit
				os.Exit(1)
			}
			panic(err)
		}
	}))
}

// fetch fetches config values from a dir defined via CLI flag --config-dir (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func fetch(printConfig bool, ignoreSettingsAtPrint ...[]string) error {
	// replace dots with underscores in env
	dotReplacer := strings.NewReplacer(".", "_")
	_node.SetEnvKeyReplacer(dotReplacer)
	// read in ENV variables
	// read in ENV variables
	_node.AutomaticEnv()

	flag.Parse()
	err := parameter.LoadConfigFile(_node, *configDirPath, *configName, true, *skipConfigAvailable)
	if err != nil {
		return err
	}

	if printConfig {
		parameter.PrintConfig(_node, ignoreSettingsAtPrint...)
	}

	for _, pluginName := range _node.GetStringSlice(node.CFG_DISABLE_PLUGINS) {
		node.DisabledPlugins[node.GetPluginIdentifier(pluginName)] = true
	}
	for _, pluginName := range _node.GetStringSlice(node.CFG_ENABLE_PLUGINS) {
		node.EnabledPlugins[node.GetPluginIdentifier(pluginName)] = true
	}

	return nil
}
