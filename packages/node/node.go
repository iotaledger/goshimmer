package node

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/daemon"
)

type Node struct {
	wg            *sync.WaitGroup
	loggers       []*Logger
	loadedPlugins []*Plugin
}

var DisabledPlugins = make(map[string]bool)
var EnabledPlugins = make(map[string]bool)

func New(plugins ...*Plugin) *Node {
	node := &Node{
		loggers:       make([]*Logger, 0),
		wg:            &sync.WaitGroup{},
		loadedPlugins: make([]*Plugin, 0),
	}

	node.AddLogger(DEFAULT_LOGGER)

	// configure the enabled plugins
	node.configure(plugins...)

	return node
}

func Start(plugins ...*Plugin) *Node {
	node := New(plugins...)
	node.Start()

	return node
}

func Run(plugins ...*Plugin) *Node {
	node := New(plugins...)
	node.Run()

	return node
}

func Shutdown() {
	daemon.ShutdownAndWait()
}

func (node *Node) AddLogger(logger *Logger) {
	node.loggers = append(node.loggers, logger)
}

func (node *Node) LogSuccess(pluginName string, message string) {
	if *LOG_LEVEL.Value >= LOG_LEVEL_SUCCESS {
		for _, logger := range node.loggers {
			if logger.Enabled {
				logger.LogSuccess(pluginName, message)
			}
		}
	}
}

func (node *Node) LogInfo(pluginName string, message string) {
	if *LOG_LEVEL.Value >= LOG_LEVEL_INFO {
		for _, logger := range node.loggers {
			if logger.Enabled {
				logger.LogInfo(pluginName, message)
			}
		}
	}
}

func (node *Node) LogDebug(pluginName string, message string) {
	if *LOG_LEVEL.Value >= LOG_LEVEL_DEBUG {
		for _, logger := range node.loggers {
			if logger.Enabled {
				logger.LogDebug(pluginName, message)
			}
		}
	}
}

func (node *Node) LogWarning(pluginName string, message string) {
	if *LOG_LEVEL.Value >= LOG_LEVEL_WARNING {
		for _, logger := range node.loggers {
			if logger.Enabled {
				logger.LogWarning(pluginName, message)
			}
		}
	}
}

func (node *Node) LogFailure(pluginName string, message string) {
	if *LOG_LEVEL.Value >= LOG_LEVEL_FAILURE {
		for _, logger := range node.loggers {
			if logger.Enabled {
				logger.LogFailure(pluginName, message)
			}
		}
	}
}

func isDisabled(plugin *Plugin) bool {
	_, exists := DisabledPlugins[GetPluginIdentifier(plugin.Name)]

	return exists
}

func isEnabled(plugin *Plugin) bool {
	_, exists := EnabledPlugins[GetPluginIdentifier(plugin.Name)]

	return exists
}

func (node *Node) configure(plugins ...*Plugin) {
	for _, plugin := range plugins {
		status := plugin.Status
		if (status == Enabled && !isDisabled(plugin)) ||
			(status == Disabled && isEnabled(plugin)) {

			plugin.wg = node.wg
			plugin.Node = node

			plugin.Events.Configure.Trigger(plugin)
			node.loadedPlugins = append(node.loadedPlugins, plugin)

			node.LogInfo("Node", "Loading Plugin: "+plugin.Name+" ... done")
		} else {
			node.LogInfo("Node", "Skipping Plugin: "+plugin.Name)
		}
	}
}

func (node *Node) Start() {
	node.LogInfo("Node", "Executing plugins ...")

	for _, plugin := range node.loadedPlugins {
		plugin.Events.Run.Trigger(plugin)

		node.LogSuccess("Node", "Starting Plugin: "+plugin.Name+" ... done")
	}

	node.LogSuccess("Node", "Starting background workers ...")

	daemon.Start()
}

func (node *Node) Run() {
	node.LogInfo("Node", "Executing plugins ...")

	for _, plugin := range node.loadedPlugins {
		plugin.Events.Run.Trigger(plugin)

		node.LogSuccess("Node", "Starting Plugin: "+plugin.Name+" ... done")
	}

	node.LogSuccess("Node", "Starting background workers ...")

	daemon.Run()

	node.LogSuccess("Node", "Shutdown complete!")
}
