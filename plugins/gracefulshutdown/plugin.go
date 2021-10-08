package gracefulshutdown

import (
	"os"
	"os/signal"
	"runtime/pprof"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the graceful shutdown plugin.
const PluginName = "GracefulShutdown"

var (
	// Plugin is the plugin instance of the graceful shutdown plugin.
	Plugin       *node.Plugin
	gracefulStop chan os.Signal
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Enabled, configure)
}

func configure(plugin *node.Plugin) {
	gracefulStop = make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		plugin.LogWarnf("Received shutdown request - waiting (max %v) to finish processing ...", Parameters.WaitToKillTime)

		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			start := time.Now()
			for x := range ticker.C {
				timeSinceStart := x.Sub(start)

				if timeSinceStart <= Parameters.WaitToKillTime {
					processList := ""
					runningBackgroundWorkers := daemon.GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						sort.Strings(runningBackgroundWorkers)
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}
					plugin.LogWarnf("Received shutdown request - waiting (max %v) to finish processing %s...", (Parameters.WaitToKillTime - timeSinceStart).Truncate(time.Second), processList)
				} else {
					plugin.LogError("Background processes did not terminate in time! Forcing shutdown ...")
					pprof.Lookup("goroutine").WriteTo(os.Stdout, 2)
					os.Exit(1)
				}
			}
		}()

		daemon.Shutdown()
	}()
}

// ShutdownWithError prints out an error message and shuts down the default daemon instance.
func ShutdownWithError(err error) {
	Plugin.LogError(err)
	gracefulStop <- syscall.SIGINT
}
