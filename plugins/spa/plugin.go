package spa

import (
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	PLUGIN = node.NewPlugin("SPA", node.Enabled, configure, run)
	log    *logger.Logger

	nodeStartAt = time.Now()

	clientsMu    sync.Mutex
	clients             = make(map[uint64]chan interface{})
	nextClientID uint64 = 0

	wsSendWorkerCount     = 1
	wsSendWorkerQueueSize = 250
	wsSendWorkerPool      *workerpool.WorkerPool
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	wsSendWorkerPool = workerpool.New(func(task workerpool.Task) {
		sendToAllWSClient(&msg{MsgTypeTPSMetric, task.Param(0).(uint64)})
		sendToAllWSClient(&msg{MsgTypeNodeStatus, currentNodeStatus()})
		sendToAllWSClient(&msg{MsgTypeNeighborMetric, neighborMetrics()})
		sendToAllWSClient(&msg{MsgTypeTipsMetric, messagelayer.TipSelector.GetTipCount()})
		task.Return(nil)
	}, workerpool.WorkerCount(wsSendWorkerCount), workerpool.QueueSize(wsSendWorkerQueueSize))

	configureLiveFeed()
	configureDrngLiveFeed()
}

func run(plugin *node.Plugin) {
	notifyStatus := events.NewClosure(func(tps uint64) {
		wsSendWorkerPool.TrySubmit(tps)
	})

	daemon.BackgroundWorker("SPA[WSSend]", func(shutdownSignal <-chan struct{}) {
		metrics.Events.ReceivedTPSUpdated.Attach(notifyStatus)
		wsSendWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[WSSend] ...")
		metrics.Events.ReceivedTPSUpdated.Detach(notifyStatus)
		wsSendWorkerPool.Stop()
		log.Info("Stopping SPA[WSSend] ... done")
	}, shutdown.PrioritySPA)

	runLiveFeed()
	runDrngLiveFeed()

	// allow any origin for websocket connections
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	if config.Node.GetBool(CFG_BASIC_AUTH_ENABLED) {
		e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == config.Node.GetString(CFG_BASIC_AUTH_USERNAME) &&
				password == config.Node.GetString(CFG_BASIC_AUTH_PASSWORD) {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(e)
	addr := config.Node.GetString(CFG_BIND_ADDRESS)

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
	MsgTypeNodeStatus byte = iota
	MsgTypeTPSMetric
	MsgTypeTx
	MsgTypeNeighborMetric
	MsgTypeDrng
	MsgTypeTipsMetric
)

type msg struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type tx struct {
	Hash  string `json:"hash"`
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
	neighbors := gossip.GetAllNeighbors()
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
