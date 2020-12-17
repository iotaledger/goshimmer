package dashboard

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/labstack/echo"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.WorkerPool

	msgHistoryMutex   sync.RWMutex
	msgSolid          map[string]bool
	msgHistory        []*tangle.Message
	maxMsgHistorySize = 1000
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID              string   `json:"id"`
	StrongParentIDs []string `json:"strongParentIDs"`
	WeakParentIDs   []string `json:"weakParentIDs"`
	IsSolid         bool     `json:"is_solid"`
}

// tipinfo holds information about whether a given message is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

// history holds a set of vertices in a DAG.
type history struct {
	Vertices []vertex `json:"vertices"`
}

func configureVisualizer() {
	visualizerWorkerPool = workerpool.New(func(task workerpool.Task) {

		switch x := task.Param(0).(type) {
		case *tangle.CachedMessage:
			sendVertex(x, task.Param(1).(*tangle.CachedMessageMetadata))
		case tangle.MessageID:
			sendTipInfo(x, task.Param(1).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))

	// configure msgHistory, msgSolid
	msgSolid = make(map[string]bool, maxMsgHistorySize)
	msgHistory = make([]*tangle.Message, 0, maxMsgHistorySize)
}

func sendVertex(cachedMessage *tangle.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	msg := cachedMessage.Unwrap()
	if msg == nil {
		return
	}
	broadcastWsMessage(&wsmsg{MsgTypeVertex, &vertex{
		ID:              msg.ID().String(),
		StrongParentIDs: msg.StrongParents().ToStrings(),
		WeakParentIDs:   msg.WeakParents().ToStrings(),
		IsSolid:         cachedMessageMetadata.Unwrap().IsSolid(),
	}}, true)
}

func sendTipInfo(messageID tangle.MessageID, isTip bool) {
	broadcastWsMessage(&wsmsg{MsgTypeTipInfo, &tipinfo{
		ID:    messageID.String(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	notifyNewMsg := events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		defer cachedMsgEvent.Message.Release()
		defer cachedMsgEvent.MessageMetadata.Release()

		// manage message history
		msg := cachedMsgEvent.Message.Unwrap()
		if msg == nil {
			return
		}
		addToHistory(msg, cachedMsgEvent.MessageMetadata.Unwrap().IsSolid())

		_, ok := visualizerWorkerPool.TrySubmit(cachedMsgEvent.Message.Retain(), cachedMsgEvent.MessageMetadata.Retain())
		if !ok {
			cachedMsgEvent.Message.Release()
			cachedMsgEvent.MessageMetadata.Release()
		}
	})

	notifyNewTip := events.NewClosure(func(messageId tangle.MessageID) {
		visualizerWorkerPool.TrySubmit(messageId, true)
	})

	notifyDeletedTip := events.NewClosure(func(messageId tangle.MessageID) {
		visualizerWorkerPool.TrySubmit(messageId, false)
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Events.MessageAttached.Attach(notifyNewMsg)
		defer messagelayer.Tangle().Events.MessageAttached.Detach(notifyNewMsg)
		messagelayer.Tangle().Events.MessageSolid.Attach(notifyNewMsg)
		defer messagelayer.Tangle().Events.MessageSolid.Detach(notifyNewMsg)
		messagelayer.TipSelector().Events.TipAdded.Attach(notifyNewTip)
		defer messagelayer.TipSelector().Events.TipAdded.Detach(notifyNewTip)
		messagelayer.TipSelector().Events.TipRemoved.Attach(notifyDeletedTip)
		defer messagelayer.TipSelector().Events.TipRemoved.Detach(notifyDeletedTip)
		visualizerWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[Visualizer] ...")
		visualizerWorkerPool.Stop()
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func setupVisualizerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/visualizer/history", func(c echo.Context) (err error) {
		msgHistoryMutex.RLock()
		cpyHistory := make([]*tangle.Message, len(msgHistory))
		copy(cpyHistory, msgHistory)
		msgHistoryMutex.RUnlock()

		var res []vertex
		for _, msg := range cpyHistory {
			res = append(res, vertex{
				ID:              msg.ID().String(),
				StrongParentIDs: msg.StrongParents().ToStrings(),
				WeakParentIDs:   msg.WeakParents().ToStrings(),
				IsSolid:         msgSolid[msg.ID().String()],
			})
		}

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}

func addToHistory(msg *tangle.Message, solid bool) {
	msgHistoryMutex.Lock()
	defer msgHistoryMutex.Unlock()
	if _, exist := msgSolid[msg.ID().String()]; exist {
		msgSolid[msg.ID().String()] = solid
		return
	}

	// remove 100 old msgs if the slice is full
	if len(msgHistory) >= maxMsgHistorySize {
		for i := 0; i < 100; i++ {
			delete(msgSolid, msgHistory[i].ID().String())
		}
		msgHistory = append(msgHistory[:0], msgHistory[100:maxMsgHistorySize]...)
	}
	// add new msg
	msgHistory = append(msgHistory, msg)
	msgSolid[msg.ID().String()] = solid
}
