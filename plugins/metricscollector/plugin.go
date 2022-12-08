package metricscollector

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"
)

// PluginName is the name of the metrics collector plugin.
const PluginName = "Metrics Collector"

var (
	// Plugin is the plugin instance of the metrics collector plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	log    *logger.Logger
	server *http.Server
)

type dependencies struct {
	dig.In

	Local     *peer.Local
	Protocol  *protocol.Protocol
	Collector *collector.Collector
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(createCollector); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// TODO do we need GoMetrics, ProcessMetrics, PromhttpMetrics

}

func run(*node.Plugin) {
	log.Info("Starting Prometheus exporter ...")

	registerMetrics()

	if err := daemon.BackgroundWorker("Prometheus exporter", func(ctx context.Context) {
		log.Info("Starting Prometheus exporter ... done")

		engine := gin.New()
		engine.Use(gin.Recovery())

		engine.GET("/metrics", func(c *gin.Context) {
			deps.Collector.Collect()

			handler := promhttp.HandlerFor(
				deps.Collector.Registry,
				promhttp.HandlerOpts{
					EnableOpenMetrics: true,
				},
			)
			handler.ServeHTTP(c.Writer, c.Request)
		})
		bindAddr := Parameters.BindAddress
		server = &http.Server{Addr: bindAddr, Handler: engine}

		go func() {
			log.Infof("You can now access the Prometheus exporter using: http://%s/metrics", bindAddr)
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("Stopping Prometheus exporter due to an error ... done")
			}
		}()

		<-ctx.Done()
		log.Info("Stopping Prometheus exporter ...")

		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := server.Shutdown(ctx)
			if err != nil {
				log.Error(err.Error())
			}
			cancel()
		}
		log.Info("Stopping Prometheus exporter ... done")

	}, shutdown.PriorityPrometheus); err != nil {
		log.Panic(err)
	}
}

func createCollector() *collector.Collector {
	return collector.New()
}

func registerMetrics() {
	fmt.Println(">>>> registering metrics...")
	deps.Collector.RegisterCollection(TangleMetrics)
	deps.Collector.RegisterCollection(ConflictMetrics)

}
