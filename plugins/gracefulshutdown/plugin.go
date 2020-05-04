package gracefulshutdown

import (
	"os"
	"os/signal"
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
const WaitToKillTimeInSeconds = 10

var log *logger.Logger
var gracefulStop chan os.Signal

// Plugin is the plugin instance of the graceful shutdown plugin.
var Plugin = node.NewPlugin(PluginName, node.Enabled, func(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	gracefulStop = make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		log.Warnf("Received shutdown request - waiting (max %d) to finish processing ...", WaitToKillTimeInSeconds)

		go func() {
			select {
			case <-time.After(WaitToKillTimeInSeconds * time.Second):
				log.Error("Background processes did not terminate in time! Forcing shutdown ...")
				os.Exit(1)
			}
		}()

		daemon.Shutdown()
	}()
})

// ShutdownWithError prints out an error message and shuts down the default daemon instance.
func ShutdownWithError(err error) {
	log.Error(err)
	gracefulStop <- syscall.SIGINT
}
