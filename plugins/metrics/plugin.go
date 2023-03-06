package metrics

// metrics is the plugin instance responsible for collection of prometheus metrics.
// All metrics should be defined in metrics_namespace.go files with different namespace for each new collection.
// Metrics naming should follow the guidelines from: https://prometheus.io/docs/practices/naming/
// In short:
// 	all metrics should be in base units, do not mix units,
// 	add suffix describing the unit,
// 	use 'total' suffix for accumulating counter

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/logger"
)

// PluginName is the name of the metrics collector plugin.
const PluginName = "Metrics"

var (
	// Plugin is the plugin instance of the metrics collector plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	log    *logger.Logger
	server *http.Server
)

type dependencies struct {
	dig.In

	Local                 *peer.Local
	Protocol              *protocol.Protocol
	BlockIssuer           *blockissuer.BlockIssuer
	P2Pmgr                *p2p.Manager        `optional:"true"`
	Selection             *selection.Protocol `optional:"true"`
	Retainer              *retainer.Retainer  `optional:"true"`
	AutopeeringConnMetric *autopeering.UDPConnTraffic

	Collector *collector.Collector
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)

	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		if err := event.Container.Provide(createCollector); err != nil {
			Plugin.Panic(err)
		}
	})
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	if Parameters.GoMetrics {
		deps.Collector.Registry.MustRegister(collectors.NewGoCollector())
	}
	if Parameters.ProcessMetrics {
		deps.Collector.Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}
}

func run(*node.Plugin) {
	log.Info("Starting Prometheus exporter ...")

	registerMetrics()

	if err := daemon.BackgroundWorker("Prometheus exporter", func(ctx context.Context) {
		log.Info("Starting Prometheus exporter ... done")

		engine := echo.New()
		engine.Use(middleware.Recover())

		engine.GET("/metrics", func(c echo.Context) error {
			deps.Collector.Collect()

			handler := promhttp.HandlerFor(
				deps.Collector.Registry,
				promhttp.HandlerOpts{
					EnableOpenMetrics: true,
				},
			)
			if Parameters.PromhttpMetrics {
				handler = promhttp.InstrumentMetricHandler(deps.Collector.Registry, handler)
			}
			handler.ServeHTTP(c.Response().Writer, c.Request())

			return nil
		})
		bindAddr := Parameters.BindAddress
		server = &http.Server{Addr: bindAddr, Handler: engine, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}

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
	deps.Collector.RegisterCollection(TangleMetrics)
	deps.Collector.RegisterCollection(ConflictMetrics)
	deps.Collector.RegisterCollection(InfoMetrics)
	deps.Collector.RegisterCollection(DBMetrics)
	deps.Collector.RegisterCollection(ManaMetrics)
	deps.Collector.RegisterCollection(AutopeeringMetrics)
	deps.Collector.RegisterCollection(RateSetterMetrics)
	deps.Collector.RegisterCollection(SchedulerMetrics)
	deps.Collector.RegisterCollection(CommitmentsMetrics)
	deps.Collector.RegisterCollection(SlotMetrics)
	deps.Collector.RegisterCollection(WorkerPoolMetrics)

}
