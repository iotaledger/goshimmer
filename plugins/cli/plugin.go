package cli

import (
	"fmt"
	"os"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/plugins/banner"
)

// PluginName is the name of the CLI plugin.
const PluginName = "CLI"

var (
	// Plugin is the plugin instance of the CLI plugin.
	Plugin  *node.Plugin
	version = flag.BoolP("version", "v", false, "Prints the GoShimmer version")
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Enabled)

	for name, plugin := range node.GetPlugins() {
		onAddPlugin(name, plugin.Status)
	}

	node.Events.AddPlugin.Attach(events.NewClosure(onAddPlugin))

	flag.Usage = printUsage

	Plugin.Events.Init.Attach(events.NewClosure(onInit))
}

func onAddPlugin(name string, status int) {
	AddPluginStatus(node.GetPluginIdentifier(name), status)
}

func onInit(_ *node.Plugin, _ *dig.Container) {
	if *version {
		fmt.Println(banner.AppName + " " + banner.AppVersion)
		os.Exit(0)
	}
}
