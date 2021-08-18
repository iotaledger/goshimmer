package prometheus

import (
	"context"
	"fmt"
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
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

// PluginName is the name of the prometheus plugin.
const PluginName = "Prometheus"

// Plugin Prometheus
var (
	Plugin *node.Plugin
	deps   dependencies
	log    *logger.Logger

	server   *http.Server
	registry = prometheus.NewRegistry()
	collects []func()
)

type dependencies struct {
	dig.In
	AutopeeringPlugin *node.Plugin `name:"autopeering"`
	Local             *peer.Local
	GossipMgr         *gossip.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	})
	if err != nil {
		fmt.Println(err)
	}

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
		if !node.IsSkipped(deps.AutopeeringPlugin) {
			registerAutopeeringMetrics()
		}
		registerDBMetrics()
		registerFPCMetrics()
		registerInfoMetrics()
		registerNetworkMetrics()
		registerProcessMetrics()
		registerTangleMetrics()
		registerManaMetrics()
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
