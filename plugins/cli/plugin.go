package cli

import (
	"flag"
	"strings"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/parameter"
)

func onAddIntParameter(param *parameter.IntParameter) {
	flagName := strings.Replace(strings.Replace(strings.ToLower(param.Name), "/", "-", 1), "_", "-", -1)

	AddIntParameter(param.Value, flagName, param.Description)
}

func onAddStringParameter(param *parameter.StringParameter) {
	flagName := strings.Replace(strings.Replace(strings.ToLower(param.Name), "/", "-", 1), "_", "-", -1)

	AddStringParameter(param.Value, flagName, param.Description)
}

func init() {
	for _, param := range parameter.GetInts() {
		onAddIntParameter(param)
	}

	for _, param := range parameter.GetStrings() {
		onAddStringParameter(param)
	}

	parameter.Events.AddInt.Attach(events.NewClosure(onAddIntParameter))
	parameter.Events.AddString.Attach(events.NewClosure(onAddStringParameter))

	flag.Usage = printUsage

	flag.Parse()
}

func configure(ctx *node.Plugin) {}

func run(plugin *node.Plugin) {}

var PLUGIN = node.NewPlugin("CLI", configure)
