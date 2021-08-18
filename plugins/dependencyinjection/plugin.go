package dependencyinjection

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/node"
)

var (
	Plugin    *node.Plugin
	Container *dig.Container
)

func init() {
	Plugin = node.NewPlugin("init", node.Enabled)
	Container = dig.New()
}
