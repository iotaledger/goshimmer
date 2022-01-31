package prometheus

import (
	"context"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/net"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

// PluginName is the name of the prometheus plugin.
const PluginName = "Prometheus"

// Plugin Prometheus
var (
	// Plugin is the plugin instance of the prometheus plugin.
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)
	deps   = new(dependencies)
	log    *logger.Logger

	server   *http.Server
	registry = prometheus.NewRegistry()
	collects []func()
)

type dependencies struct {
	dig.In
	AutopeeringPlugin     *node.Plugin `name:"autopeering" optional:"true"`
	Local                 *peer.Local
	GossipMgr             *gossip.Manager `optional:"true"`
	AutoPeeringConnMetric *net.ConnMetric `optional:"true"`
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	if Parameters.WorkerpoolMetrics {
		registerWorkerpoolMetrics()
	}

	if Parameters.GoMetrics {
		registry.MustRegister(prometheus.NewGoCollector())
	}
	if Parameters.ProcessMetrics {
		registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	}

	if metrics.Parameters.Local {
		if deps.AutopeeringPlugin != nil {
			registerAutopeeringMetrics()
		}
		registerDBMetrics()
		registerInfoMetrics()
		registerNetworkMetrics()
		registerProcessMetrics()
		registerTangleMetrics()
		registerManaMetrics()
		registerSchedulerMetrics()
	}

	if metrics.Parameters.Global {
		registerClientsMetrics()
	}

	if metrics.Parameters.ManaResearch {
		registerManaResearchMetrics()
	}
}

func addCollect(collect func()) {
	collects = append(collects, collect)
}

func run(*node.Plugin) {
	log.Info("Starting Prometheus exporter ...")

	if err := daemon.BackgroundWorker("Prometheus exporter", func(ctx context.Context) {
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
			if Parameters.PromhttpMetrics {
				handler = promhttp.InstrumentMetricHandler(registry, handler)
			}
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
