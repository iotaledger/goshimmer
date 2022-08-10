package bootstrapmanager

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/core/bootstrapmanager"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// Plugin is the plugin instance of the blocklayer plugin.
	Plugin     *node.Plugin
	_          = new(dependencies)
	pluginDeps = new(pluginDependencies)
)

type dependencies struct {
	dig.In

	Tangle          *tangleold.Tangle
	NotarizationMgr *notarization.Manager
}

type pluginDependencies struct {
	dig.In

	Tangle           *tangleold.Tangle
	NotarizationMgr  *notarization.Manager
	BootstrapManager *bootstrapmanager.Manager
}

func init() {
	Plugin = node.NewPlugin("BootstrapManager", pluginDeps, node.Enabled, configure)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newManager); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	pluginDeps.BootstrapManager.Setup()
}

func newManager(deps dependencies) *bootstrapmanager.Manager {
	return bootstrapmanager.New(
		deps.Tangle,
		deps.NotarizationMgr,
	)
}
