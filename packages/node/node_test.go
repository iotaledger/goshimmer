package node_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/app/configuration"
	"github.com/iotaledger/hive.go/app/logger"
)

func TestDependencyInjection(t *testing.T) {
	type PluginADeps struct {
		dig.In
		DepFromB string `name:"providedByB"`
	}

	stringVal := "到月球"

	depsA := &PluginADeps{}
	pluginA := node.NewPlugin("A", depsA, node.Enabled,
		func(plugin *node.Plugin) {
			require.Equal(t, stringVal, depsA.DepFromB)
		},
	)

	pluginB := node.NewPlugin("B", nil, node.Enabled)

	pluginB.Events.Init.Hook(func(event *node.InitEvent) {
		require.NoError(t, event.Container.Provide(func() string {
			return stringVal
		}, dig.Name("providedByB")))
	})

	require.NoError(t, logger.InitGlobalLogger(configuration.New()))
	node.Run(node.Plugins(pluginA, pluginB))
}
