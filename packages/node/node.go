package node

import (
	"fmt"
	"reflect"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/logger"
)

var (
	// plugins.
	plugins         = make(map[string]*Plugin)
	DisabledPlugins = make(map[string]bool)
	EnabledPlugins  = make(map[string]bool)
)

type Node struct {
	loadedPlugins []*Plugin
	Logger        *logger.Logger
	options       *NodeOptions
	depContainer  *dig.Container
}

func New(optionalOptions ...NodeOption) *Node {
	node := &Node{
		loadedPlugins: make([]*Plugin, 0),
		options:       newNodeOptions(optionalOptions),
		depContainer:  dig.New(),
	}

	// initialize plugins
	node.init(node.options.plugins...)

	// initialize logger after init phase because plugins could modify it
	node.Logger = logger.NewLogger("Node")

	// configure the enabled plugins
	node.configure(node.options.plugins...)

	return node
}

func Run(optionalOptions ...NodeOption) *Node {
	node := New(optionalOptions...)
	node.Run()

	return node
}

func Shutdown() {
	daemon.ShutdownAndWait()
}

// IsSkipped returns whether the plugin is loaded or skipped.
func IsSkipped(plugin *Plugin) bool {
	return (plugin.Status == Disabled || isDisabled(plugin)) &&
		(plugin.Status == Enabled || !isEnabled(plugin))
}

func isDisabled(plugin *Plugin) bool {
	_, exists := DisabledPlugins[GetPluginIdentifier(plugin.Name)]

	return exists
}

func isEnabled(plugin *Plugin) bool {
	_, exists := EnabledPlugins[GetPluginIdentifier(plugin.Name)]

	return exists
}

func (node *Node) init(plugins ...*Plugin) {
	for _, plugin := range plugins {
		if IsSkipped(plugin) {
			continue
		}
		plugin.Events.Init.Trigger(&InitEvent{plugin, node.depContainer})
	}
}

func (node *Node) configure(plugins ...*Plugin) {
	for _, plugin := range plugins {
		if IsSkipped(plugin) {
			node.Logger.Infof("Skipping Plugin: %s", plugin.Name)

			continue
		}

		plugin.Node = node

		if plugin.deps != nil {
			node.populatePluginDependencies(plugin)
		}

		plugin.Events.Configure.Trigger(&ConfigureEvent{plugin})
		node.loadedPlugins = append(node.loadedPlugins, plugin)
		node.Logger.Infof("Loading Plugin: %s ... done", plugin.Name)
	}
}

func (node *Node) populatePluginDependencies(plugin *Plugin) {
	depsType := reflect.TypeOf(plugin.deps)
	if depsType.Kind() != reflect.Ptr {
		panic("must pass pointer to plugin dependency struct")
	}

	depStructVal := reflect.Indirect(reflect.ValueOf(plugin.deps))
	depStructType := depStructVal.Type()

	invokeFnType := reflect.FuncOf([]reflect.Type{depStructType}, []reflect.Type{}, false)
	invokeFn := reflect.MakeFunc(invokeFnType, func(args []reflect.Value) (results []reflect.Value) {
		reflect.ValueOf(plugin.deps).Elem().Set(args[0])

		return results
	})

	if err := node.depContainer.Invoke(invokeFn.Interface()); err != nil {
		panic(fmt.Errorf("unable to populate dependencies of plugin %s: %w", plugin.Name, err))
	}
}

func (node *Node) Run() {
	node.Logger.Info("Executing plugins ...")

	for _, plugin := range node.loadedPlugins {
		plugin.WorkerPool.Start()
		plugin.Events.Run.Trigger(&RunEvent{plugin})
		node.Logger.Infof("Starting Plugin: %s ... done", plugin.Name)
	}

	node.Logger.Info("Starting background workers ...")

	daemon.Run()

	for _, plugin := range node.loadedPlugins {
		plugin.WorkerPool.Shutdown()
	}

	node.Logger.Info("Shutdown complete!")
}

func (node *Node) LoadedPlugins() []*Plugin {
	return node.loadedPlugins
}

func AddPlugin(plugin *Plugin) {
	name := plugin.Name
	status := plugin.Status

	if _, exists := plugins[name]; exists {
		panic("duplicate plugin - \"" + name + "\" was defined already")
	}

	plugins[name] = plugin

	Events.AddPlugin.Trigger(&AddEvent{name, status})
}

func GetPlugins() map[string]*Plugin {
	return plugins
}
