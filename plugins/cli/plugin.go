package cli

import (
	"fmt"
	"strings"

	"github.com/iotaledger/hive.go/events"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/parameter"
)

const (
	// AppVersion version number
	AppVersion = "v0.0.1"
	// AppName app code name
	AppName = "GoShimmer"
)

var PLUGIN = node.NewPlugin("CLI", node.Enabled, configure, run)

func onAddPlugin(name string, status int) {
	AddPluginStatus(node.GetPluginIdentifier(name), status)
}

func init() {

	for name, status := range parameter.GetPlugins() {
		onAddPlugin(name, status)
	}

	parameter.Events.AddPlugin.Attach(events.NewClosure(onAddPlugin))

	flag.Usage = printUsage
}

func parseParameters() {
	for _, pluginName := range parameter.NodeConfig.GetStringSlice(node.CFG_DISABLE_PLUGINS) {
		node.DisabledPlugins[strings.ToLower(pluginName)] = true
	}
	for _, pluginName := range parameter.NodeConfig.GetStringSlice(node.CFG_ENABLE_PLGUINS) {
		node.EnabledPlugins[strings.ToLower(pluginName)] = true
	}
}

func configure(ctx *node.Plugin) {

	fmt.Println("  _____ _   _ ________  ______  ___ ___________ ")
	fmt.Println(" /  ___| | | |_   _|  \\/  ||  \\/  ||  ___| ___ \\")
	fmt.Println(" \\ `--.| |_| | | | | .  . || .  . || |__ | |_/ /")
	fmt.Println("  `--. \\  _  | | | | |\\/| || |\\/| ||  __||    / ")
	fmt.Println(" /\\__/ / | | |_| |_| |  | || |  | || |___| |\\ \\ ")
	fmt.Printf(" \\____/\\_| |_/\\___/\\_|  |_/\\_|  |_/\\____/\\_| \\_| fullnode %s", AppVersion)
	fmt.Println()

	parameter.FetchConfig()
	parseParameters()

	ctx.Node.LogInfo("Node", "Loading plugins ...")
}

func run(ctx *node.Plugin) {
	// do nothing; everything is handled in the configure step
}
