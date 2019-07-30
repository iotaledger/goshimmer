package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/parameter"
)

func onAddBoolParameter(param *parameter.BoolParameter) {
	flagName := strings.Replace(strings.Replace(strings.ToLower(param.Name), "/", "-", 1), "_", "-", -1)

	AddBoolParameter(param.Value, flagName, param.Description)
}

func onAddIntParameter(param *parameter.IntParameter) {
	flagName := strings.Replace(strings.Replace(strings.ToLower(param.Name), "/", "-", 1), "_", "-", -1)

	AddIntParameter(param.Value, flagName, param.Description)
}

func onAddStringParameter(param *parameter.StringParameter) {
	flagName := strings.Replace(strings.Replace(strings.ToLower(param.Name), "/", "-", 1), "_", "-", -1)

	AddStringParameter(param.Value, flagName, param.Description)
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

	parameter.Events.AddBool.Attach(events.NewClosure(onAddBoolParameter))
	parameter.Events.AddInt.Attach(events.NewClosure(onAddIntParameter))
	parameter.Events.AddString.Attach(events.NewClosure(onAddStringParameter))

	flag.Usage = printUsage
}

func configure(ctx *node.Plugin) {
	flag.Parse()

	for _, disabledPlugin := range strings.Fields(*node.DISABLE_PLUGINS.Value) {
		node.DisabledPlugins[strings.ToLower(disabledPlugin)] = true
	}

	fmt.Println("  _____ _   _ ________  ______  ___ ___________ ")
	fmt.Println(" /  ___| | | |_   _|  \\/  ||  \\/  ||  ___| ___ \\")
	fmt.Println(" \\ `--.| |_| | | | | .  . || .  . || |__ | |_/ /")
	fmt.Println("  `--. \\  _  | | | | |\\/| || |\\/| ||  __||    / ")
	fmt.Println(" /\\__/ / | | |_| |_| |  | || |  | || |___| |\\ \\ ")
	fmt.Println(" \\____/\\_| |_/\\___/\\_|  |_/\\_|  |_/\\____/\\_| \\_| fullnode 0.0.1")
	fmt.Println()

	ctx.Node.LogInfo("Node", "Loading plugins ...")
}

var PLUGIN = node.NewPlugin("CLI", configure, func(plugin *node.Plugin) {

})
