package cli

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

var PLUGIN = node.NewPlugin("CLI", node.Enabled, run)

func onAddPlugin(name string, status int) {
	AddPluginStatus(node.GetPluginIdentifier(name), status)
}

func init() {
	for name, status := range node.GetPlugins() {
		onAddPlugin(name, status)
	}

	node.Events.AddPlugin.Attach(events.NewClosure(onAddPlugin))

	flag.Usage = printUsage
}

func run(ctx *node.Plugin) {
	// do nothing; everything is handled in the init method
}
