package cli

import (
	"fmt"
	"os"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"

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
		onAddPlugin(&node.AddEvent{Name: name, Status: plugin.Status})
	}

	node.Events.AddPlugin.Hook(event.NewClosure[*node.AddEvent](onAddPlugin))

	flag.Usage = printUsage

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](onInit))
}

func onAddPlugin(addEvent *node.AddEvent) {
	AddPluginStatus(node.GetPluginIdentifier(addEvent.Name), addEvent.Status)
}

func onInit(_ *node.InitEvent) {
	if *version {
		fmt.Println(banner.AppName + " " + banner.AppVersion)
		os.Exit(0)
	}
}
