package metricscollector

import (
	"context"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/app/metricscollector"
	"github.com/iotaledger/goshimmer/packages/app/metricscollector/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"

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

	Local *peer.Local
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// TODO do we need GoMetrics, ProcessMetrics, PromhttpMetrics

}

func run(*node.Plugin) {
	log.Info("Starting Prometheus exporter ...")

	collector := metricscollector.NewCollector()
	registerMetrics(collector)

	if err := daemon.BackgroundWorker("Prometheus exporter", func(ctx context.Context) {
		log.Info("Starting Prometheus exporter ... done")

		engine := gin.New()
		engine.Use(gin.Recovery())

		engine.GET("/metrics", func(c *gin.Context) {
			collector.Collect()

			handler := promhttp.HandlerFor(
				collector.Registry,
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

func registerMetrics(c *metricscollector.Collector) {
	c.RegisterCollection(metrics.TangleMetrics)
	c.RegisterCollection(metrics.ConflictMetrics)

}
