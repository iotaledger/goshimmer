package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/parameter"
)

func getFlagName(paramName string) string {
	return strings.Replace(strings.Replace(strings.ToLower(paramName), "/", "-", 1), "_", "-", -1)
}

func onAddBoolParameter(param *parameter.BoolParameter) {
	AddBoolParameter(param.Value, getFlagName(param.Name), param.Description)
}

func onAddIntParameter(param *parameter.IntParameter) {
	AddIntParameter(param.Value, getFlagName(param.Name), param.Description)
}

func onAddStringParameter(param *parameter.StringParameter) {
	AddStringParameter(param.Value, getFlagName(param.Name), param.Description)
}

func onAddPlugin(name string, status int) {
	AddPluginStatus(node.GetPluginIdentifier(name), status)
}

func init() {
	for _, param := range parameter.GetBools() {
		onAddBoolParameter(param)
	}
	for _, param := range parameter.GetInts() {
		onAddIntParameter(param)
	}
	for _, param := range parameter.GetStrings() {
		onAddStringParameter(param)
	}
	for name, status := range parameter.GetPlugins() {
		onAddPlugin(name, status)
	}

	parameter.Events.AddBool.Attach(events.NewClosure(onAddBoolParameter))
	parameter.Events.AddInt.Attach(events.NewClosure(onAddIntParameter))
	parameter.Events.AddString.Attach(events.NewClosure(onAddStringParameter))
	parameter.Events.AddPlugin.Attach(events.NewClosure(onAddPlugin))

	flag.Usage = printUsage
}

func parseParameters() {
	for _, pluginName := range strings.Fields(*node.DISABLE_PLUGINS.Value) {
		node.DisabledPlugins[strings.ToLower(pluginName)] = true
	}
	for _, pluginName := range strings.Fields(*node.ENABLE_PLUGINS.Value) {
		node.EnabledPlugins[strings.ToLower(pluginName)] = true
	}
}

func configure(ctx *node.Plugin) {
	flag.Parse()

	parseParameters()

	fmt.Println("  _____ _   _ ________  ______  ___ ___________ ")
	fmt.Println(" /  ___| | | |_   _|  \\/  ||  \\/  ||  ___| ___ \\")
	fmt.Println(" \\ `--.| |_| | | | | .  . || .  . || |__ | |_/ /")
	fmt.Println("  `--. \\  _  | | | | |\\/| || |\\/| ||  __||    / ")
	fmt.Println(" /\\__/ / | | |_| |_| |  | || |  | || |___| |\\ \\ ")
	fmt.Println(" \\____/\\_| |_/\\___/\\_|  |_/\\_|  |_/\\____/\\_| \\_| fullnode 0.0.1")
	fmt.Println()

	ctx.Node.LogInfo("Node", "Loading plugins ...")
}

func run(ctx *node.Plugin) {
	// do nothing; everything is handled in the configure step
}

var PLUGIN = node.NewPlugin("CLI", node.Enabled, configure, run)
