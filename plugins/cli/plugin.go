package cli

import (
	"fmt"
	"os"

	"github.com/iotaledger/hive.go/generics/event"
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

	node.Events.AddPlugin.Attach(event.NewClosure(func(event *node.AddEvent) { onAddPlugin(event.Name, event.Status) }))

	flag.Usage = printUsage

	Plugin.Events.Init.Attach(event.NewClosure(func(event *node.InitEvent) { onInit(event.Plugin, event.Container) }))
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
