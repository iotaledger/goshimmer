// Package remotelog is a plugin that enables log messages being sent via UDP to a central ELK stack for debugging.
// It is disabled by default and when enabled, additionally, logger.disableEvents=false in config.json needs to be set.
// The destination can be set via logger.remotelog.serverAddress.
// All events according to logger.level in config.json are sent.
package remotelog

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
	flag "github.com/spf13/pflag"
	"gopkg.in/src-d/go-git.v4"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/config"
)

const (
	// CfgLoggerRemotelogServerAddress defines the config flag of the server address.
	CfgLoggerRemotelogServerAddress = "logger.remotelog.serverAddress"
	// CfgDisableEvents defines the config flag for disabling logger events.
	CfgDisableEvents = "logger.disableEvents"
	// PluginName is the name of the remote log plugin.
	PluginName = "RemoteLog"

	remoteLogType = "log"

	levelIndex   = 0
	nameIndex    = 1
	messageIndex = 2
)

var (
	// plugin is the plugin instance of the remote plugin instance.
	plugin      *node.Plugin
	pluginOnce  sync.Once
	myID        string
	myGitHead   string
	myGitBranch string
	workerPool  *workerpool.NonBlockingQueuedWorkerPool

	remoteLogger     *RemoteLoggerConn
	remoteLoggerOnce sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

func init() {
	flag.String(CfgLoggerRemotelogServerAddress, "ressims.iota.cafe:5213", "RemoteLog server address")
}

func configure(plugin *node.Plugin) {
	if config.Node().Bool(CfgDisableEvents) {
		plugin.LogFatalf("%s in config.json needs to be false so that events can be captured!", CfgDisableEvents)
		return
	}

	// initialize remote logger connection
	RemoteLogger()

	if local.GetInstance() != nil {
		myID = local.GetInstance().ID().String()
	}

	getGitInfo()

	workerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		SendLogMsg(task.Param(levelIndex).(logger.Level), task.Param(nameIndex).(string), task.Param(messageIndex).(string))

		task.Return(nil)
	}, workerpool.WorkerCount(runtime.GOMAXPROCS(0)), workerpool.QueueSize(1000))
}

func run(plugin *node.Plugin) {
	logEvent := events.NewClosure(func(level logger.Level, name string, msg string) {
		workerPool.TrySubmit(level, name, msg)
	})

	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		logger.Events.AnyMsg.Attach(logEvent)
		<-shutdownSignal
		plugin.LogInfof("Stopping %s ...", PluginName)
		logger.Events.AnyMsg.Detach(logEvent)
		workerPool.Stop()
		plugin.LogInfof("Stopping %s ... done", PluginName)
	}, shutdown.PriorityRemoteLog); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

// SendLogMsg sends log message to the remote logger.
func SendLogMsg(level logger.Level, name, msg string) {
	m := logMessage{
		banner.AppVersion,
		myGitHead,
		myGitBranch,
		myID,
		level.CapitalString(),
		name,
		msg,
		clock.SyncedTime(),
		remoteLogType,
	}

	_ = RemoteLogger().Send(m)
}

func getGitInfo() {
	r, err := git.PlainOpen(getGitDir())
	if err != nil {
		plugin.LogDebug("Could not open Git repo.")
		return
	}

	// extract git branch and head
	if h, err := r.Head(); err == nil {
		myGitBranch = h.Name().String()
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

// RemoteLogger represents a connection to our remote log server.
func RemoteLogger() *RemoteLoggerConn {
	remoteLoggerOnce.Do(func() {
		r, err := newRemoteLoggerConn(config.Node().String(CfgLoggerRemotelogServerAddress))
		if err != nil {
			plugin.LogFatal(err)
			return
		}

		remoteLogger = r
	})

	return remoteLogger
}

type logMessage struct {
	Version   string    `json:"version"`
	GitHead   string    `json:"gitHead,omitempty"`
	GitBranch string    `json:"gitBranch,omitempty"`
	NodeID    string    `json:"nodeId"`
	Level     string    `json:"level"`
	Name      string    `json:"name"`
	Msg       string    `json:"msg"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
}
