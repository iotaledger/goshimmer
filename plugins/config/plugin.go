package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

// PluginName is the name of the config plugin.
const PluginName = "Config"

var (
	// plugin is the plugin instance of the config plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once

	// flags
	defaultConfigName   = "config.json"
	configFilePath      = flag.StringP("config", "c", defaultConfigName, "file path of the config file")
	skipConfigAvailable = flag.Bool("skip-config", false, "Skip config file availability check")

	// Node is viper
	_node    *configuration.Configuration
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
func Node() *configuration.Configuration {
	nodeOnce.Do(func() {
		_node = configuration.New()
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

// fetch fetches config values from a configFilePath (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func fetch(printConfig bool, ignoreSettingsAtPrint ...[]string) error {
	flag.Parse()

	if err := _node.LoadFile(*configFilePath); err != nil {
		if hasFlag(defaultConfigName) {
			// if a file was explicitly specified, raise the error
			fmt.Println("config error")
			return err
		}
		fmt.Printf("No config file found via '%s'. Loading default settings.", *configFilePath)
	}

	if err := _node.LoadFlagSet(flag.CommandLine); err != nil {
		return err
	}

	// read in ENV variables
	// load the env vars after default values from flags were set (otherwise the env vars are not added because the keys don't exist)
	if err := _node.LoadEnvironmentVars(""); err != nil {
		return err
	}

	// load the flags again to overwrite env vars that were also set via command line
	if err := _node.LoadFlagSet(flag.CommandLine); err != nil {
		return err
	}

	if printConfig {
		PrintConfig(ignoreSettingsAtPrint...)
	}

	for _, pluginName := range _node.Strings(CfgDisablePlugins) {
		node.DisabledPlugins[node.GetPluginIdentifier(pluginName)] = true
	}
	for _, pluginName := range _node.Strings(CfgEnablePlugins) {
		node.EnabledPlugins[node.GetPluginIdentifier(pluginName)] = true
	}

	return nil
}

// PrintConfig prints the config.
func PrintConfig(ignoreSettingsAtPrint ...[]string) {
	_node.Print(ignoreSettingsAtPrint...)
}

func hasFlag(name string) bool {
	has := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			has = true
		}
	})
	return has
}
