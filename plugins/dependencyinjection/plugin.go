package dependencyinjection

import (
	"fmt"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/node"
)

var (
	Plugin    *node.Plugin
	Container *dig.Container
)

func init() {
	fmt.Println("dependency injection init")
	Plugin = node.NewPlugin("init", node.Enabled)
	Container = dig.New()
}
