package gracefulshutdown

import (
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
)

// maximum amount of time to wait for background processes to terminate. After that the process is killed.
const WAIT_TO_KILL_TIME_IN_SECONDS = 10

var PLUGIN = node.NewPlugin("Graceful Shutdown", func(plugin *node.Plugin) {
	gracefulStop := make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		plugin.LogWarning("Received shutdown request - waiting (max " + strconv.Itoa(WAIT_TO_KILL_TIME_IN_SECONDS) + " seconds) to finish processing ...")

		go func() {
			start := time.Now()
			for x := range time.Tick(1 * time.Second) {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= WAIT_TO_KILL_TIME_IN_SECONDS {
					processList := ""
					runningBackgroundWorkers := daemon.GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}

					plugin.LogWarning("Received shutdown request - waiting (max " + strconv.Itoa(WAIT_TO_KILL_TIME_IN_SECONDS-int(secondsSinceStart)) + " seconds) to finish processing " + processList + "...")
				} else {
					plugin.LogFailure("Background processes did not terminate in time! Forcing shutdown ...")

					os.Exit(1)
				}
			}
		}()

		daemon.Shutdown()
	}()
})
