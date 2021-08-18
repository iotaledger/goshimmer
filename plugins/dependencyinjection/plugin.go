package dependencyinjection

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/node"
)

var (
	// Plugin is the plugin instance of the dependency injection plugin.
	Plugin *node.Plugin
	// Container is a dependency injection container.
	Container *dig.Container
)

func init() {
	Plugin = node.NewPlugin("init", node.Enabled)
	Container = dig.New()
}
