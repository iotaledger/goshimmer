package cli

import (
	"fmt"
	"os"
	"sync"

	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

// PluginName is the name of the CLI plugin.
const PluginName = "CLI"

var (
	// plugin is the plugin instance of the CLI plugin.
	plugin  *node.Plugin
	once    sync.Once
	version = flag.BoolP("version", "v", false, "Prints the GoShimmer version")
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled)
	})
	return plugin
}

func init() {
	plugin = Plugin()

	for name, plugin := range node.GetPlugins() {
		onAddPlugin(name, plugin.Status)
	}

	node.Events.AddPlugin.Attach(events.NewClosure(onAddPlugin))

	flag.Usage = printUsage

	plugin.Events.Init.Attach(events.NewClosure(onInit))
}

func onAddPlugin(name string, status int) {
	AddPluginStatus(node.GetPluginIdentifier(name), status)
}

func onInit(*node.Plugin) {
	if *version {
		fmt.Println(banner.AppName + " " + banner.AppVersion)
		os.Exit(0)
	}
}
