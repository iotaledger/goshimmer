package node

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const (
	Disabled = iota
	Enabled
)

type Callback = func(plugin *Plugin)

type Plugin struct {
	Node       *Node
	Name       string
	Status     int
	Events     *PluginEvents
	log        *logger.Logger
	logOnce    sync.Once
	deps       interface{}
	WorkerPool *workerpool.WorkerPool
}

// NewPlugin creates a new plugin with the given name, default status and callbacks.
// The last specified callback is the mandatory run callback, while all other callbacks are configure callbacks.
func NewPlugin(name string, deps interface{}, status int, callbacks ...Callback) *Plugin {
	plugin := &Plugin{
		Name:       name,
		Status:     status,
		deps:       deps,
		Events:     newPluginEvents(),
		WorkerPool: workerpool.New(fmt.Sprintf("Plugin-%s", name), 1),
	}

	AddPlugin(plugin)

	switch len(callbacks) {
	case 0:
		// plugin doesn't have any callbacks (i.e. plugins that execute stuff on init())
	case 1:
		plugin.Events.Run.Hook(func(event *RunEvent) { callbacks[0](event.Plugin) })
	case 2:
		plugin.Events.Configure.Hook(func(event *ConfigureEvent) { callbacks[0](event.Plugin) })
		plugin.Events.Run.Hook(func(event *RunEvent) { callbacks[1](event.Plugin) })
	default:
		panic("too many callbacks in NewPlugin(...)")
	}

	return plugin
}

func GetPluginIdentifier(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", ""))
}

// LogDebug uses fmt.Sprint to construct and log a message.
func (p *Plugin) LogDebug(args ...interface{}) {
	p.Logger().Debug(args...)
}

// LogDebugf uses fmt.Sprintf to log a templated message.
func (p *Plugin) LogDebugf(format string, args ...interface{}) {
	p.Logger().Debugf(format, args...)
}

// LogError uses fmt.Sprint to construct and log a message.
func (p *Plugin) LogError(args ...interface{}) {
	p.Logger().Error(args...)
}

// LogErrorAndExit uses fmt.Sprint to construct and log a message, then calls os.Exit.
func (p *Plugin) LogErrorAndExit(args ...interface{}) {
	p.Logger().Error(args...)
	p.Logger().Error("Exiting...")
	os.Exit(1)
}

// LogErrorf uses fmt.Sprintf to log a templated message.
func (p *Plugin) LogErrorf(format string, args ...interface{}) {
	p.Logger().Errorf(format, args...)
}

// LogErrorfAndExitf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func (p *Plugin) LogErrorfAndExitf(format string, args ...interface{}) {
	p.Logger().Errorf(format, args...)
	p.Logger().Error("Exiting...")
	os.Exit(1)
}

// LogFatalAndExit uses fmt.Sprint to construct and log a message, then calls os.Exit.
func (p *Plugin) LogFatalAndExit(args ...interface{}) {
	p.Logger().Fatal(args...)
}

// LogFatalfAndExitf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func (p *Plugin) LogFatalfAndExitf(format string, args ...interface{}) {
	p.Logger().Fatalf(format, args...)
}

// LogInfo uses fmt.Sprint to construct and log a message.
func (p *Plugin) LogInfo(args ...interface{}) {
	p.Logger().Info(args...)
}

// LogInfof uses fmt.Sprintf to log a templated message.
func (p *Plugin) LogInfof(format string, args ...interface{}) {
	p.Logger().Infof(format, args...)
}

// LogWarn uses fmt.Sprint to construct and log a message.
func (p *Plugin) LogWarn(args ...interface{}) {
	p.Logger().Warn(args...)
}

// LogWarnf uses fmt.Sprintf to log a templated message.
func (p *Plugin) LogWarnf(format string, args ...interface{}) {
	p.Logger().Warnf(format, args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func (p *Plugin) Panic(args ...interface{}) {
	p.Logger().Panic(args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func (p *Plugin) Panicf(template string, args ...interface{}) {
	p.Logger().Panicf(template, args...)
}

// Logger instantiates and returns a logger with the name of the plugin.
func (p *Plugin) Logger() *logger.Logger {
	p.logOnce.Do(func() {
		p.log = logger.NewLogger(p.Name)
	})

	return p.log
}
