package dashboard

import (
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// PluginName is the name of the dashboard plugin.
const PluginName = "Dashboard"

var (
	// Plugin is the plugin instance of the dashboard plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	log    *logger.Logger

	nodeStartAt = time.Now()

	clientsMu    sync.Mutex
	clients      = make(map[uint64]chan interface{})
	nextClientID uint64

	wsSendWorkerCount     = 1
	wsSendWorkerQueueSize = 250
	wsSendWorkerPool      *workerpool.WorkerPool
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	wsSendWorkerPool = workerpool.New(func(task workerpool.Task) {
		sendToAllWSClient(&wsmsg{MsgTypeMPSMetric, task.Param(0).(uint64)})
		sendToAllWSClient(&wsmsg{MsgTypeNodeStatus, currentNodeStatus()})
		sendToAllWSClient(&wsmsg{MsgTypeNeighborMetric, neighborMetrics()})
		sendToAllWSClient(&wsmsg{MsgTypeTipsMetric, messagelayer.TipSelector.TipCount()})
		task.Return(nil)
	}, workerpool.WorkerCount(wsSendWorkerCount), workerpool.QueueSize(wsSendWorkerQueueSize))

	configureLiveFeed()
	configureDrngLiveFeed()
}

func run(plugin *node.Plugin) {
	notifyStatus := events.NewClosure(func(mps uint64) {
		wsSendWorkerPool.TrySubmit(mps)
	})

	daemon.BackgroundWorker("Dashboard[WSSend]", func(shutdownSignal <-chan struct{}) {
		metrics.Events.ReceivedMPSUpdated.Attach(notifyStatus)
		wsSendWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[WSSend] ...")
		metrics.Events.ReceivedMPSUpdated.Detach(notifyStatus)
		wsSendWorkerPool.Stop()
		log.Info("Stopping Dashboard[WSSend] ... done")
	}, shutdown.PriorityDashboard)

	runLiveFeed()
	runDrngLiveFeed()

	// allow any origin for websocket connections
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	if config.Node.GetBool(CfgBasicAuthEnabled) {
		e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == config.Node.GetString(CfgBasicAuthUsername) &&
				password == config.Node.GetString(CfgBasicAuthPassword) {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(e)
	addr := config.Node.GetString(CfgBindAddress)

	log.Infof("You can now access the dashboard using: http://%s", addr)
	go e.Start(addr)
}

// sends the given message to all connected websocket clients
func sendToAllWSClient(msg interface{}) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for _, channel := range clients {
		select {
		case channel <- msg:
		default:
			// drop if buffer not drained
		}
	}
}

var webSocketWriteTimeout = time.Duration(3) * time.Second

var (
	upgrader = websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		EnableCompression: true,
	}
)

const (
	// MsgTypeNodeStatus is the type of the NodeStatus message.
	MsgTypeNodeStatus byte = iota
	// MsgTypeMPSMetric is the type of the message per second (MPS) metric message.
	MsgTypeMPSMetric
	// MsgTypeMessage is the type of the message.
	MsgTypeMessage
	// MsgTypeNeighborMetric is the type of the NeighborMetric message.
	MsgTypeNeighborMetric
	// MsgTypeDrng is the type of the dRNG message.
	MsgTypeDrng
	// MsgTypeTipsMetric is the type of the TipsMetric message.
	MsgTypeTipsMetric
)

type wsmsg struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type msg struct {
	ID    string `json:"id"`
	Value int64  `json:"value"`
}

type drngMsg struct {
	Instance      uint32 `json:"instance"`
	DistributedPK string `json:"dpk"`
	Round         uint64 `json:"round"`
	Randomness    string `json:"randomness"`
	Timestamp     string `json:"timestamp"`
}

type nodestatus struct {
	ID      string      `json:"id"`
	Version string      `json:"version"`
	Uptime  int64       `json:"uptime"`
	Mem     *memmetrics `json:"mem"`
}

type memmetrics struct {
	Sys          uint64 `json:"sys"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	MSpanInuse   uint64 `json:"m_span_inuse"`
	MCacheInuse  uint64 `json:"m_cache_inuse"`
	StackSys     uint64 `json:"stack_sys"`
	NumGC        uint32 `json:"num_gc"`
	LastPauseGC  uint64 `json:"last_pause_gc"`
}

type neighbormetric struct {
	ID               string `json:"id"`
	Address          string `json:"address"`
	ConnectionOrigin string `json:"connection_origin"`
	BytesRead        uint32 `json:"bytes_read"`
	BytesWritten     uint32 `json:"bytes_written"`
}

func neighborMetrics() []neighbormetric {
	stats := []neighbormetric{}

	// gossip plugin might be disabled
	neighbors := gossip.Neighbors()
	if neighbors == nil {
		return stats
	}

	for _, neighbor := range neighbors {
		// unfortunately the neighbor manager doesn't keep track of the origin of the connection
		origin := "Inbound"
		for _, peer := range autopeering.Selection.GetOutgoingNeighbors() {
			if neighbor.Peer == peer {
				origin = "Outbound"
				break
			}
		}

		host := neighbor.Peer.IP().String()
		port := neighbor.Peer.Services().Get(service.GossipKey).Port()
		stats = append(stats, neighbormetric{
			ID:               neighbor.Peer.ID().String(),
			Address:          net.JoinHostPort(host, strconv.Itoa(port)),
			BytesRead:        neighbor.BytesRead(),
			BytesWritten:     neighbor.BytesWritten(),
			ConnectionOrigin: origin,
		})
	}
	return stats
}

func currentNodeStatus() *nodestatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status := &nodestatus{}
	status.ID = local.GetInstance().ID().String()

	// node status
	status.Version = banner.AppVersion
	status.Uptime = time.Since(nodeStartAt).Milliseconds()

	// memory metrics
	status.Mem = &memmetrics{
		Sys:          m.Sys,
		HeapSys:      m.HeapSys,
		HeapInuse:    m.HeapInuse,
		HeapIdle:     m.HeapIdle,
		HeapReleased: m.HeapReleased,
		HeapObjects:  m.HeapObjects,
		MSpanInuse:   m.MSpanInuse,
		MCacheInuse:  m.MCacheInuse,
		StackSys:     m.StackSys,
		NumGC:        m.NumGC,
		LastPauseGC:  m.PauseNs[(m.NumGC+255)%256],
	}
	return status
}
