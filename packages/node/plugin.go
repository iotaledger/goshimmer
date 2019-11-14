package node

import (
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/parameter"
)

const (
	Disabled = iota
	Enabled
)

type Plugin struct {
	Node   *Node
	Name   string
	Status int
	Events pluginEvents
	wg     *sync.WaitGroup
}

// Creates a new plugin with the given name, default status and callbacks.
// The last specified callback is the mandatory run callback, while all other callbacks are configure callbacks.
func NewPlugin(name string, status int, callback Callback, callbacks ...Callback) *Plugin {
	plugin := &Plugin{
		Name:   name,
		Status: status,
		Events: pluginEvents{
			Configure: events.NewEvent(pluginCaller),
			Run:       events.NewEvent(pluginCaller),
		},
	}

	// make the plugin known to the parameters
	parameter.AddPlugin(name, status)

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

func GetPluginIdentifier(name string) string {
	return strings.ToLower(strings.Replace(name, " ", "", -1))
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
