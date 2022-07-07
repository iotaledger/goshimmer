package dashboard

import (
	"context"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/chat"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

// TODO: mana visualization + metrics

// PluginName is the name of the dashboard plugin.
const PluginName = "Dashboard"

var (
	// Plugin is the plugin instance of the dashboard plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	log    *logger.Logger
	server *echo.Echo

	nodeStartAt = time.Now()
)

type dependencies struct {
	dig.In

	Node      *configuration.Configuration
	Local     *peer.Local
	Tangle    *tangle.Tangle
	Selection *selection.Protocol `optional:"true"`
	GossipMgr *gossip.Manager     `optional:"true"`
	Chat      *chat.Chat          `optional:"true"`
	Indexer   *indexer.Indexer
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	configureWebSocketWorkerPool()
	configureLiveFeed()
	configureChatLiveFeed()
	configureVisualizer()
	configureManaFeed()
	configureServer()
	configureConflictLiveFeed()
}

func configureServer() {
	server = echo.New()
	server.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))
	server.HideBanner = true
	server.HidePort = true
	server.Use(middleware.Recover())

	if Parameters.BasicAuth.Enabled {
		server.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == Parameters.BasicAuth.Username &&
				password == Parameters.BasicAuth.Password {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(server)
}

func run(*node.Plugin) {
	// run block broker
	runWebSocketStreams()
	// run the block live feed
	runLiveFeed()
	// run the visualizer vertex feed
	runVisualizer()
	runManaFeed()
	runConflictLiveFeed()

	if deps.Chat != nil {
		runChatLiveFeed()
	}

	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker(PluginName, worker, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Error starting as daemon: %s", err)
	}
}

func worker(ctx context.Context) {
	defer log.Infof("Stopping %s ... done", PluginName)

	defer wsSendWorkerPool.Stop()

	// submit the mps to the worker pool when triggered
	notifyStatus := event.NewClosure(func(event *metrics.ReceivedBPSUpdatedEvent) { wsSendWorkerPool.TrySubmit(event.BPS) })
	metrics.Events.ReceivedBPSUpdated.Attach(notifyStatus)
	defer metrics.Events.ReceivedBPSUpdated.Detach(notifyStatus)

	stopped := make(chan struct{})
	go func() {
		log.Infof("%s started, bind-address=%s, basic-auth=%v", PluginName, Parameters.BindAddress, Parameters.BasicAuth.Enabled)
		if err := server.Start(Parameters.BindAddress); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error serving: %s", err)
			}
			close(stopped)
		}
	}()

	// stop if we are shutting down or the server could not be started
	select {
	case <-ctx.Done():
	case <-stopped:
	}

	log.Infof("Stopping %s ...", PluginName)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
}

const (
	// BlkTypeNodeStatus is the type of the NodeStatus block.
	BlkTypeNodeStatus byte = iota
	// BlkTypeBPSMetric is the type of the block per second (BPS) metric block.
	BlkTypeBPSMetric
	// BlkTypeBlock is the type of the block.
	BlkTypeBlock
	// BlkTypeNeighborMetric is the type of the NeighborMetric block.
	BlkTypeNeighborMetric
	// BlkTypeComponentCounterMetric is the type of the component counter triggered per second.
	BlkTypeComponentCounterMetric
	// BlkTypeTipsMetric is the type of the TipsMetric block.
	BlkTypeTipsMetric
	// BlkTypeVertex defines a vertex block.
	BlkTypeVertex
	// BlkTypeTipInfo defines a tip info block.
	BlkTypeTipInfo
	// BlkTypeManaValue defines a mana value block.
	BlkTypeManaValue
	// BlkTypeManaMapOverall defines a block containing overall mana map.
	BlkTypeManaMapOverall
	// BlkTypeManaMapOnline defines a block containing online mana map.
	BlkTypeManaMapOnline
	// BlkTypeManaAllowedPledge defines a block containing a list of allowed mana pledge nodeIDs.
	BlkTypeManaAllowedPledge
	// BlkTypeManaPledge defines a block that is sent when mana was pledged to the node.
	BlkTypeManaPledge
	// BlkTypeManaInitPledge defines a block that is sent when initial pledge events are sent to the dashboard.
	BlkTypeManaInitPledge
	// BlkTypeManaRevoke defines a block that is sent when mana was revoked from a node.
	BlkTypeManaRevoke
	// BlkTypeManaInitRevoke defines a block that is sent when initial revoke events are sent to the dashboard.
	BlkTypeManaInitRevoke
	// BlkTypeManaInitDone defines a block that is sent when all initial values are sent.
	BlkTypeManaInitDone
	// BlkManaDashboardAddress is the socket address of the dashboard to stream mana from.
	BlkManaDashboardAddress
	// BlkTypeChat defines a chat block.
	BlkTypeChat
	// BlkTypeRateSetterMetric defines rate setter metrics.
	BlkTypeRateSetterMetric
	// BlkTypeConflictsConflict defines a block that contains a conflictSet update for the conflictSet tab.
	BlkTypeConflictsConflict
	// BlkTypeConflictsBranch defines a block that contains a branch update for the conflictSet tab.
	BlkTypeConflictsBranch
)

type wsblk struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type blk struct {
	ID          string `json:"id"`
	Value       int64  `json:"value"`
	PayloadType uint32 `json:"payload_type"`
}

type nodestatus struct {
	ID         string          `json:"id"`
	Version    string          `json:"version"`
	Uptime     int64           `json:"uptime"`
	Mem        *memmetrics     `json:"mem"`
	TangleTime tangleTime      `json:"tangleTime"`
	Scheduler  schedulerMetric `json:"scheduler"`
}

type tangleTime struct {
	Synced       bool  `json:"synced"`
	Bootstrapped bool  `json:"bootstrapped"`
	ATT          int64 `json:"ATT"`
	RATT         int64 `json:"RATT"`
	CTT          int64 `json:"CTT"`
	RCTT         int64 `json:"RCTT"`

	AcceptedBlockID  string `json:"acceptedBlockID"`
	ConfirmedBlockID string `json:"confirmedBlockID"`
}

type memmetrics struct {
	HeapSys      uint64 `json:"heap_sys"`
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	NumGC        uint32 `json:"num_gc"`
	LastPauseGC  uint64 `json:"last_pause_gc"`
}

type neighbormetric struct {
	ID               string `json:"id"`
	Address          string `json:"address"`
	ConnectionOrigin string `json:"connection_origin"`
	PacketsRead      uint64 `json:"packets_read"`
	PacketsWritten   uint64 `json:"packets_written"`
}

type tipsInfo struct {
	TotalTips int `json:"totaltips"`
}

type componentsmetric struct {
	Store      uint64 `json:"store"`
	Solidifier uint64 `json:"solidifier"`
	Scheduler  uint64 `json:"scheduler"`
	Booker     uint64 `json:"booker"`
}

type rateSetterMetric struct {
	Size     int     `json:"size"`
	Estimate string  `json:"estimate"`
	Rate     float64 `json:"rate"`
}

type schedulerMetric struct {
	Running           bool    `json:"running"`
	Rate              string  `json:"rate"`
	MaxBufferSize     int     `json:"maxBufferSize"`
	CurrentBufferSize int     `json:"currentBufferSize"`
	Deficit           float64 `json:"deficit"`
}

func neighborMetrics() []neighbormetric {
	var stats []neighbormetric
	if deps.GossipMgr == nil {
		return stats
	}

	// gossip plugin might be disabled
	neighbors := deps.GossipMgr.AllNeighbors()
	if neighbors == nil {
		return stats
	}

	for _, neighbor := range neighbors {
		// unfortunately the neighbor manager doesn't keep track of the origin of the connection
		// TODO: kinda a hack, the manager should keep track of the direction of the connection
		origin := "Inbound"
		if deps.Selection != nil {
			for _, peer := range deps.Selection.GetOutgoingNeighbors() {
				if neighbor.Peer == peer {
					origin = "Outbound"
					break
				}
			}
		}

		host := neighbor.Peer.IP().String()
		port := neighbor.Peer.Services().Get(service.GossipKey).Port()
		stats = append(stats, neighbormetric{
			ID:               neighbor.Peer.ID().String(),
			Address:          net.JoinHostPort(host, strconv.Itoa(port)),
			PacketsRead:      neighbor.PacketsRead(),
			PacketsWritten:   neighbor.PacketsWritten(),
			ConnectionOrigin: origin,
		})
	}
	return stats
}

func currentNodeStatus() *nodestatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status := &nodestatus{}
	status.ID = deps.Local.ID().String()

	// node status
	status.Version = banner.AppVersion
	status.Uptime = time.Since(nodeStartAt).Milliseconds()

	// memory metrics
	status.Mem = &memmetrics{
		HeapSys:      m.HeapSys,
		HeapAlloc:    m.HeapAlloc,
		HeapIdle:     m.HeapIdle,
		HeapReleased: m.HeapReleased,
		HeapObjects:  m.HeapObjects,
		NumGC:        m.NumGC,
		LastPauseGC:  m.PauseNs[(m.NumGC+255)%256],
	}

	// get TangleTime
	tm := deps.Tangle.TimeManager
	status.TangleTime = tangleTime{
		Synced:           tm.Synced(),
		Bootstrapped:     tm.Bootstrapped(),
		AcceptedBlockID:  tm.LastAcceptedBlock().BlockID.Base58(),
		ConfirmedBlockID: tm.LastConfirmedBlock().BlockID.Base58(),
		ATT:              tm.ATT().UnixNano(),
		RATT:             tm.RATT().UnixNano(),
		CTT:              tm.CTT().UnixNano(),
		RCTT:             tm.RCTT().UnixNano(),
	}

	deficit, _ := deps.Tangle.Scheduler.GetDeficit(deps.Local.ID()).Float64()

	status.Scheduler = schedulerMetric{
		Running:           deps.Tangle.Scheduler.Running(),
		Rate:              deps.Tangle.Scheduler.Rate().String(),
		MaxBufferSize:     deps.Tangle.Scheduler.MaxBufferSize(),
		CurrentBufferSize: deps.Tangle.Scheduler.BufferSize(),
		Deficit:           deficit,
	}
	return status
}
