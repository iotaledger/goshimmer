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
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/chat"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/gossip"
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

	Node         *configuration.Configuration
	Local        *peer.Local
	Tangle       *tangle.Tangle
	Selection    *selection.Protocol `optional:"true"`
	GossipMgr    *gossip.Manager     `optional:"true"`
	DRNGInstance *drng.DRNG          `optional:"true"`
	Chat         *chat.Chat          `optional:"true"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	configureWebSocketWorkerPool()
	configureLiveFeed()
	configureDrngLiveFeed()
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
	// run message broker
	runWebSocketStreams()
	// run the message live feed
	runLiveFeed()
	// run the visualizer vertex feed
	runVisualizer()
	runManaFeed()
	runConflictLiveFeed()
	if deps.DRNGInstance != nil {
		runDrngLiveFeed()
	}

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
	notifyStatus := events.NewClosure(func(mps uint64) { wsSendWorkerPool.TrySubmit(mps) })
	metrics.Events.ReceivedMPSUpdated.Attach(notifyStatus)
	defer metrics.Events.ReceivedMPSUpdated.Detach(notifyStatus)

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
	// MsgTypeNodeStatus is the type of the NodeStatus message.
	MsgTypeNodeStatus byte = iota
	// MsgTypeMPSMetric is the type of the message per second (MPS) metric message.
	MsgTypeMPSMetric
	// MsgTypeMessage is the type of the message.
	MsgTypeMessage
	// MsgTypeNeighborMetric is the type of the NeighborMetric message.
	MsgTypeNeighborMetric
	// MsgTypeComponentCounterMetric is the type of the component counter triggered per second.
	MsgTypeComponentCounterMetric
	// MsgTypeDrng is the type of the dRNG message.
	MsgTypeDrng
	// MsgTypeTipsMetric is the type of the TipsMetric message.
	MsgTypeTipsMetric
	// MsgTypeVertex defines a vertex message.
	MsgTypeVertex
	// MsgTypeTipInfo defines a tip info message.
	MsgTypeTipInfo
	// MsgTypeManaValue defines a mana value message.
	MsgTypeManaValue
	// MsgTypeManaMapOverall defines a message containing overall mana map.
	MsgTypeManaMapOverall
	// MsgTypeManaMapOnline defines a message containing online mana map.
	MsgTypeManaMapOnline
	// MsgTypeManaAllowedPledge defines a message containing a list of allowed mana pledge nodeIDs.
	MsgTypeManaAllowedPledge
	// MsgTypeManaPledge defines a message that is sent when mana was pledged to the node.
	MsgTypeManaPledge
	// MsgTypeManaInitPledge defines a message that is sent when initial pledge events are sent to the dashboard.
	MsgTypeManaInitPledge
	// MsgTypeManaRevoke defines a message that is sent when mana was revoked from a node.
	MsgTypeManaRevoke
	// MsgTypeManaInitRevoke defines a message that is sent when initial revoke events are sent to the dashboard.
	MsgTypeManaInitRevoke
	// MsgTypeManaInitDone defines a message that is sent when all initial values are sent.
	MsgTypeManaInitDone
	// MsgManaDashboardAddress is the socket address of the dashboard to stream mana from.
	MsgManaDashboardAddress
	// MsgTypeMsgOpinionFormed defines a tip info message.
	MsgTypeMsgOpinionFormed
	// MsgTypeChat defines a chat message.
	MsgTypeChat
	// MsgTypeConflictsConflict defines a message that contains a conflict update for the conflict tab.
	MsgTypeConflictsConflict
	// MsgTypeConflictsBranch defines a message that contains a branch update for the conflict tab.
	MsgTypeConflictsBranch
)

type wsmsg struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type msg struct {
	ID          string `json:"id"`
	Value       int64  `json:"value"`
	PayloadType uint32 `json:"payload_type"`
}

type nodestatus struct {
	ID         string      `json:"id"`
	Version    string      `json:"version"`
	Uptime     int64       `json:"uptime"`
	Mem        *memmetrics `json:"mem"`
	TangleTime tangleTime  `json:"tangleTime"`
}

type tangleTime struct {
	Synced    bool   `json:"synced"`
	Time      int64  `json:"time"`
	MessageID string `json:"messageID"`
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
	lcm := deps.Tangle.TimeManager.LastConfirmedMessage()
	status.TangleTime = tangleTime{
		Synced:    deps.Tangle.TimeManager.Synced(),
		Time:      lcm.Time.UnixNano(),
		MessageID: lcm.MessageID.Base58(),
	}
	return status
}
