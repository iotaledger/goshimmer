package gracefulshutdown

import (
	"os"
	"os/signal"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the graceful shutdown plugin.
const PluginName = "Graceful Shutdown"

var (
	// plugin is the plugin instance of the graceful shutdown plugin.
	plugin       *node.Plugin
	once         sync.Once
	gracefulStop chan os.Signal
)

func configure(*node.Plugin) {
	gracefulStop = make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		Plugin().LogWarnf("Received shutdown request - waiting (max %d) to finish processing ...", Parameters.WaitToKillTime)

		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			start := time.Now()
			for x := range ticker.C {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= float64(Parameters.WaitToKillTime) {
					processList := ""
					runningBackgroundWorkers := daemon.GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						sort.Strings(runningBackgroundWorkers)
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}
					Plugin().LogWarnf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", Parameters.WaitToKillTime-int(secondsSinceStart), processList)
				} else {
					Plugin().LogError("Background processes did not terminate in time! Forcing shutdown ...")
					pprof.Lookup("goroutine").WriteTo(os.Stdout, 2)
					os.Exit(1)
				}
			}
		}()

		daemon.Shutdown()
	}()
}

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

// ShutdownWithError prints out an error message and shuts down the default daemon instance.
func ShutdownWithError(err error) {
	Plugin().LogError(err)
	gracefulStop <- syscall.SIGINT
}
