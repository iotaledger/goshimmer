package prometheus

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Plugin Prometheus
var (
	Plugin = node.NewPlugin("Prometheus", node.Disabled, configure, run)
	log    *logger.Logger

	server   *http.Server
	registry = prometheus.NewRegistry()
	collects []func()
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	if config.Node().GetBool(CfgPrometheusGoMetrics) {
		registry.MustRegister(prometheus.NewGoCollector())
	}
	if config.Node().GetBool(CfgPrometheusProcessMetrics) {
		registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	}

	if config.Node().GetBool(metrics.CfgMetricsLocal) {
		registerAutopeeringMetrics()
		registerDBMetrics()
		registerFPCMetrics()
		registerInfoMetrics()
		registerNetworkMetrics()
		registerProcessMetrics()
		registerTangleMetrics()
	}

	if config.Node().GetBool(metrics.CfgMetricsGlobal) {
		registerClientsMetrics()
	}
}

func addCollect(collect func()) {
	collects = append(collects, collect)
}

func run(plugin *node.Plugin) {
	log.Info("Starting Prometheus exporter ...")

	if err := daemon.BackgroundWorker("Prometheus exporter", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Prometheus exporter ... done")

		engine := gin.New()
		engine.Use(gin.Recovery())
		engine.GET("/metrics", func(c *gin.Context) {
			for _, collect := range collects {
				collect()
			}
			handler := promhttp.HandlerFor(
				registry,
				promhttp.HandlerOpts{
					EnableOpenMetrics: true,
				},
			)
			if config.Node().GetBool(CfgPrometheusPromhttpMetrics) {
				handler = promhttp.InstrumentMetricHandler(registry, handler)
			}
			handler.ServeHTTP(c.Writer, c.Request)
		})

		bindAddr := config.Node().GetString(CfgPrometheusBindAddress)
		server = &http.Server{Addr: bindAddr, Handler: engine}

		go func() {
			log.Infof("You can now access the Prometheus exporter using: http://%s/metrics", bindAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("Stopping Prometheus exporter due to an error ... done")
			}
		}()

		<-shutdownSignal
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
