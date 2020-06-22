package gracefulshutdown

import (
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the graceful shutdown plugin.
const PluginName = "Graceful Shutdown"

// WaitToKillTimeInSeconds is the maximum amount of time to wait for background processes to terminate.
// After that the process is killed.
const WaitToKillTimeInSeconds = 60

var log *logger.Logger
var gracefulStop chan os.Signal

// plugin is the plugin instance of the graceful shutdown plugin.
var plugin = node.NewPlugin(PluginName, node.Enabled, func(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	gracefulStop = make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		log.Warnf("Received shutdown request - waiting (max %d) to finish processing ...", WaitToKillTimeInSeconds)

		go func() {
			start := time.Now()
			for x := range time.Tick(1 * time.Second) {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= WaitToKillTimeInSeconds {
					processList := ""
					runningBackgroundWorkers := daemon.GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						sort.Strings(runningBackgroundWorkers)
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}
					log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", WaitToKillTimeInSeconds-int(secondsSinceStart), processList)
				} else {
					log.Error("Background processes did not terminate in time! Forcing shutdown ...")
					os.Exit(1)
				}
			}
		}()

		daemon.Shutdown()
	}()
})

// Plugin gets the plugin instance
func Plugin() *node.Plugin {
	return plugin
}


// ShutdownWithError prints out an error message and shuts down the default daemon instance.
func ShutdownWithError(err error) {
	log.Error(err)
	gracefulStop <- syscall.SIGINT
}
