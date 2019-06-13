package node

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/events"
)

type Plugin struct {
	Node   *Node
	Name   string
	Events pluginEvents
	wg     *sync.WaitGroup
}

func NewPlugin(name string, callback Callback, callbacks ...Callback) *Plugin {
	plugin := &Plugin{
		Name: name,
		Events: pluginEvents{
			Configure: events.NewEvent(pluginCaller),
			Run:       events.NewEvent(pluginCaller),
		},
	}

	if len(callbacks) >= 1 {
		plugin.Events.Configure.Attach(events.NewClosure(callback))
		for _, callback = range callbacks[:len(callbacks)-1] {
			plugin.Events.Configure.Attach(events.NewClosure(callback))
		}

		plugin.Events.Run.Attach(events.NewClosure(callbacks[len(callbacks)-1]))
	} else {
		plugin.Events.Run.Attach(events.NewClosure(callback))
	}

	return plugin
}

func (plugin *Plugin) LogSuccess(message string) {
	plugin.Node.LogSuccess(plugin.Name, message)
}

func (plugin *Plugin) LogInfo(message string) {
	plugin.Node.LogInfo(plugin.Name, message)
}

func (plugin *Plugin) LogWarning(message string) {
	plugin.Node.LogWarning(plugin.Name, message)
}

func (plugin *Plugin) LogFailure(message string) {
	plugin.Node.LogFailure(plugin.Name, message)
}

func (plugin *Plugin) LogDebug(message string) {
	plugin.Node.LogDebug(plugin.Name, message)
}
