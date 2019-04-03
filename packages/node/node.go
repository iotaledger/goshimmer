package node

import (
    "fmt"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "strings"
    "sync"
)

type Node struct {
    wg            *sync.WaitGroup
    loggers       []*Logger
    loadedPlugins []*Plugin
    logLevel      int
}

var disabledPlugins = make(map[string]bool)

func Load(plugins ...*Plugin) *Node {
    for _, disabledPlugin := range strings.Fields(*DISABLE_PLUGINS.Value) {
        disabledPlugins[strings.ToLower(disabledPlugin)] = true
    }

    fmt.Println("  _____ _   _ ________  ______  ___ ___________ ")
    fmt.Println(" /  ___| | | |_   _|  \\/  ||  \\/  ||  ___| ___ \\")
    fmt.Println(" \\ `--.| |_| | | | | .  . || .  . || |__ | |_/ /")
    fmt.Println("  `--. \\  _  | | | | |\\/| || |\\/| ||  __||    / ")
    fmt.Println(" /\\__/ / | | |_| |_| |  | || |  | || |___| |\\ \\ ")
    fmt.Println(" \\____/\\_| |_/\\___/\\_|  |_/\\_|  |_/\\____/\\_| \\_| fullnode 1.0")
    fmt.Println()

    node := &Node{
        logLevel:      *LOG_LEVEL.Value,
        loggers:       make([]*Logger, 0),
        wg:            &sync.WaitGroup{},
        loadedPlugins: make([]*Plugin, 0),
    }

    node.AddLogger(DEFAULT_LOGGER)
    node.Load(plugins...)

    return node
}

func Run(plugins ...*Plugin) *Node {
    node := Load(plugins...)
    node.Run()

    return node
}

func (node *Node) AddLogger(logger *Logger) {
    node.loggers = append(node.loggers, logger)
}

func (node *Node) LogSuccess(pluginName string, message string) {
    if node.logLevel >= LOG_LEVEL_SUCCESS {
        for _, logger := range node.loggers {
            if logger.Enabled {
                logger.LogSuccess(pluginName, message)
            }
        }
    }
}

func (node *Node) LogInfo(pluginName string, message string) {
    if node.logLevel >= LOG_LEVEL_INFO {
        for _, logger := range node.loggers {
            if logger.Enabled {
                logger.LogInfo(pluginName, message)
            }
        }
    }
}

func (node *Node) LogDebug(pluginName string, message string) {
    if node.logLevel >= LOG_LEVEL_DEBUG {
        for _, logger := range node.loggers {
            if logger.Enabled {
                logger.LogDebug(pluginName, message)
            }
        }
    }
}

func (node *Node) LogWarning(pluginName string, message string) {
    if node.logLevel >= LOG_LEVEL_WARNING {
        for _, logger := range node.loggers {
            if logger.Enabled {
                logger.LogWarning(pluginName, message)
            }
        }
    }
}

func (node *Node) LogFailure(pluginName string, message string) {
    if node.logLevel >= LOG_LEVEL_FAILURE {
        for _, logger := range node.loggers {
            if logger.Enabled {
                logger.LogFailure(pluginName, message)
            }
        }
    }
}

func (node *Node) Load(plugins ...*Plugin) {
    node.LogInfo("Node", "Loading plugins ...")

    if len(plugins) >= 1 {
        for _, plugin := range plugins {
            if _, exists := disabledPlugins[strings.ToLower(strings.Replace(plugin.Name, " ", "", -1))]; !exists {
                plugin.wg = node.wg
                plugin.Node = node

                plugin.Events.Configure.Trigger(plugin)

                node.LogInfo("Node", "Loading Plugin: " + plugin.Name + " ... done")

                node.loadedPlugins = append(node.loadedPlugins, plugin)
            }
        }
    }

    //node.loadedPlugins = append(node.loadedPlugins, plugins...)
}

func (node *Node) Run() {
    node.LogInfo("Node", "Executing plugins ...")

    if len(node.loadedPlugins) >= 1 {
        for _, plugin := range node.loadedPlugins {
            plugin.Events.Run.Trigger(plugin)

            node.LogSuccess("Node", "Starting Plugin: "+plugin.Name+" ... done")
        }
    }

    node.LogSuccess("Node", "Starting background workers ...")

    daemon.Run()

    node.LogSuccess("Node", "Shutdown complete!")
}
