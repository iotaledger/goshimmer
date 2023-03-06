// Package remotelog is a plugin that enables log blocks being sent via UDP to a central ELK stack for debugging.
// It is disabled by default and when enabled, additionally, logger.disableEvents=false in config.json needs to be set.
// The destination can be set via logger.remotelog.serverAddress.
// All events according to logger.level in config.json are sent.
package remotelog

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/dig"
	"gopkg.in/src-d/go-git.v4"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	logger_plugin "github.com/iotaledger/goshimmer/plugins/logger"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/runtime/event"
)

const (
	// PluginName is the name of the remote log plugin.
	PluginName = "RemoteLog"

	remoteLogType = "log"

	levelIndex = 0
	nameIndex  = 1
	blockIndex = 2
)

var (
	// Plugin is the plugin instance of the remote plugin instance.
	Plugin        *node.Plugin
	deps          = new(dependencies)
	myID          string
	myGitHead     string
	myGitConflict string
)

type dependencies struct {
	dig.In

	Local        *peer.Local
	RemoteLogger *RemoteLoggerConn
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		if err := event.Container.Provide(func() *RemoteLoggerConn {
			remoteLogger, err := newRemoteLoggerConn(Parameters.RemoteLog.ServerAddress)
			if err != nil {
				Plugin.LogFatalAndExit(err)
				return nil
			}
			return remoteLogger
		}); err != nil {
			Plugin.Panic(err)
		}
	})
}

func configure(_ *node.Plugin) {
	if logger_plugin.Parameters.DisableEvents {
		return
	}

	if deps.Local != nil {
		myID = deps.Local.ID().String()
	}

	getGitInfo()
}

func run(plugin *node.Plugin) {
	if logger_plugin.Parameters.DisableEvents {
		return
	}

	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		hook := logger.Events.AnyMsg.Hook(func(logEvent *logger.LogEvent) {
			deps.RemoteLogger.SendLogMsg(logEvent.Level, logEvent.Name, logEvent.Msg)
		}, event.WithWorkerPool(plugin.WorkerPool))
		<-ctx.Done()
		plugin.LogInfof("Stopping %s ...", PluginName)
		hook.Unhook()
		plugin.LogInfof("Stopping %s ... done", PluginName)
	}, shutdown.PriorityRemoteLog); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func getGitInfo() {
	r, err := git.PlainOpen(getGitDir())
	if err != nil {
		Plugin.LogDebug("Could not open Git repo.")
		return
	}

	// extract git conflict and head
	if h, err := r.Head(); err == nil {
		myGitConflict = h.Name().String()
		myGitHead = h.Hash().String()
	}
}

func getGitDir() string {
	var gitDir string

	// this is valid when running an executable, when using "go run" this is a temp path
	if ex, err := os.Executable(); err == nil {
		temp := filepath.Join(filepath.Dir(ex), ".git")
		if _, err := os.Stat(temp); err == nil {
			gitDir = temp
		}
	}

	// when running "go run" from the same directory
	if gitDir == "" {
		if wd, err := os.Getwd(); err == nil {
			temp := filepath.Join(wd, ".git")
			if _, err := os.Stat(temp); err == nil {
				gitDir = temp
			}
		}
	}

	return gitDir
}

type logBlock struct {
	Version     string    `json:"version"`
	GitHead     string    `json:"gitHead,omitempty"`
	GitConflict string    `json:"gitConflict,omitempty"`
	NodeID      string    `json:"nodeId"`
	Level       string    `json:"level"`
	Name        string    `json:"name"`
	Msg         string    `json:"msg"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
}
